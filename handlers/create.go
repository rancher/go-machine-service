package handlers

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"bytes"

	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
	"github.com/rancher/event-subscriber/events"
	"github.com/rancher/go-machine-service/handlers/providers"
	v3 "github.com/rancher/go-rancher/v3"
	"golang.org/x/net/context"
)

const (
	errorCreatingMachine = "Error creating machine: "
	createdFile          = "created"
	bootstrapContName    = "rancher-agent-bootstrap"
	parseMessage         = "Failed to parse config: [%v]"
	defaultVersion       = "1.22"
)

var regExHyphen = regexp.MustCompile("([a-z])([A-Z])")

var endpointRegEx = regexp.MustCompile("-H=[[:alnum:]]*[[:graph:]]*")

func CreateMachineAndActivateMachine(event *events.Event, apiClient *v3.RancherClient) error {
	log := logger.WithFields(logrus.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
	})

	//Setup republishing timer
	publishChan := make(chan string, 10)
	go republishTransitioningReply(publishChan, event, apiClient)
	defer close(publishChan)

	host, hostDir, err := getHostAndHostDir(event, apiClient)
	if err != nil || host == nil {
		return err
	}
	defer os.RemoveAll(hostDir)
	// first check if host is already created. If so we restore the config
	restored, err := isHostAlreadyCreated(host, hostDir)
	if err != nil {
		return err
	}

	if !restored {
		if err := createMachine(event, apiClient, publishChan, log); err != nil {
			return err
		}
	}
	if err := registerRancherAgent(event, apiClient, publishChan); err != nil {
		return err
	}
	if err := touchBootstrappedStamp(hostDir, host); err != nil {
		return err
	}

	return publishReply(newReply(event), apiClient)
}

func createMachine(event *events.Event, apiClient *v3.RancherClient, publishChan chan string, log *logrus.Entry) error {
	log.Info("Creating Host")
	machineCreated := false
	host, hostDir, err := getHostAndHostDir(event, apiClient)
	if err != nil || host == nil {
		return err
	}
	if err := applyHostTemplate(host, apiClient); err != nil {
		return err
	}
	if _, err := os.Stat(createdStamp(hostDir, host)); !os.IsNotExist(err) {
		return publishReply(newReply(event), apiClient)
	}
	defer func() {
		if !machineCreated {
			cleanupResources(hostDir, host.Hostname)
		}
	}()

	hostTemplate, err := apiClient.HostTemplate.ById(host.HostTemplateId)
	if err != nil {
		return err
	}
	driver := hostTemplate.Driver

	providerHandler := providers.GetProviderHandler(driver)
	if err := providerHandler.HandleCreate(host, hostDir); err != nil {
		return err
	}

	command, err := buildCreateCommand(host, hostDir, driver)
	if err != nil {
		return err
	}

	publishChan <- "Contacting " + driver

	readerStdout, readerStderr, err := startReturnOutput(command)
	if err != nil {
		return err
	}

	errChan := make(chan string, 1)
	go logProgress(readerStdout, readerStderr, publishChan, host, event, errChan, providerHandler)

	if err := command.Wait(); err != nil {
		select {
		case errString := <-errChan:
			if errString != "" {
				return fmt.Errorf(errString)
			}
		case <-time.After(10 * time.Second):
			log.Error("Waited 10 seconds to break after command.Wait().  Please review logProgress.")
		}
		return err
	}

	log.Info("Machine Created")
	machineCreated = true
	touchCreatedStamp(hostDir, host)

	destFile, err := createExtractedConfig(hostDir, host)
	if err != nil {
		return err
	}

	extractedConf, err := encodeFile(destFile)
	if err != nil {
		return err
	}

	for i := 0; i < 100; i++ {
		_, err = apiClient.Host.Update(host, &v3.Host{
			ExtractedConfig: extractedConf,
		})
		if err == nil {
			break
		}
	}
	log.Info("Machine config file saved.")
	return nil
}

func registerRancherAgent(event *events.Event, apiClient *v3.RancherClient, publishChan chan string) error {
	logger.WithFields(logrus.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
	}).Info("Activating Machine")

	publishChan <- "Installing Rancher agent"
	host, hostDir, err := getHostAndHostDir(event, apiClient)
	if err != nil || host == nil {
		return err
	}

	registrationURL, imageRepo, imageTag, err := getRegistrationURLAndImage(host.ClusterId, apiClient)
	if err != nil {
		return err
	}

	dockerClient, err := GetDockerClient(hostDir, host.Hostname)
	if err != nil {
		return err
	}

	err = pullImage(dockerClient, imageRepo, imageTag)
	if err != nil {
		return err
	}

	publishChan <- "Creating agent container"

	contID, err := createContainer(registrationURL, host, dockerClient, imageRepo, imageTag)
	if err != nil {
		return err
	}
	logger.WithFields(logrus.Fields{
		"resourceId":  event.ResourceID,
		"machineId":   host.Id,
		"containerId": contID,
	}).Info("Container created for machine")

	publishChan <- "Starting agent container"

	err = dockerClient.ContainerStart(context.Background(), contID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	found := false
	for i := 0; i < 30; i++ {
		containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
		if err != nil {
			return err
		}
		for _, c := range containers {
			if len(c.Names) > 0 && c.Names[0] == "/rancher-agent" {
				found = true
				break
			}
		}
		time.Sleep(2 * time.Second)
	}

	if !found {
		logger.WithFields(logrus.Fields{
			"resourceId": event.ResourceID,
			"machineId":  host.Id,
		}).Error("Failed to find rancher-agent container")
		return errors.New("Failed to find rancher-agent container")
	}

	go func() {
		accountID, err := getAccountID(host, apiClient)
		if err != nil {
			return
		}

		images, err := collectImageNames(accountID, apiClient)
		if err != nil {
			return
		}
		for _, image := range images {
			repo, tag, err := parseImage(image)
			if err != nil {
				continue
			}
			logger.WithFields(logrus.Fields{
				"resourceId":        event.ResourceID,
				"machineExternalId": host.Uuid,
				"repo":              repo,
				"tag":               tag,
			}).Info("Pulling image")
			go pullImage(dockerClient, repo, tag)
		}
	}()

	publishChan <- "Waiting for agent initialization"

	foundAgentID := false
	for i := 0; i < 150; i++ {
		host, err := apiClient.Host.ById(host.Id)
		if err != nil {
			logrus.Errorf("failed to get host. err: %v", err)
			continue
		}
		if host.AgentId != "" {
			foundAgentID = true
			break
		}
		time.Sleep(2 * time.Second)
	}

	if !foundAgentID {
		logrus.Errorf("host is not registered correctly. ResourceId: %v, hostId: %v", event.ResourceID, host.Id)
		return errors.New("Host is not registered correctly")
	}

	// swallow the error as we don't care if it is deleted or not
	dockerClient.ContainerRemove(context.Background(), contID, types.ContainerRemoveOptions{Force: true})

	logger.WithFields(logrus.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": host.Uuid,
		"containerId":       contID,
	}).Info("Rancher-agent for machine started")
	return nil
}

func isHostAlreadyCreated(host *v3.Host, hostDir string) (bool, error) {
	if host.ExtractedConfig == "" {
		return false, nil
	}
	err := restoreMachineDir(host, hostDir)
	if err != nil {
		return false, err
	}
	mExists, err := machineExists(hostDir, host.Hostname)
	if err != nil {
		return false, err
	}
	if mExists {

	}
	return true, nil
}

func getAccountID(host *v3.Host, apiClient *v3.RancherClient) (string, error) {
	accounts, err := apiClient.Account.List(&v3.ListOpts{
		Filters: map[string]interface{}{
			"clusterId":    host.ClusterId,
			"clusterOwner": true,
		},
	})
	if err != nil {
		return "", err
	}

	if len(accounts.Data) != 1 {
		return "", fmt.Errorf("Failed to find account for host, %d found", len(accounts.Data))
	}

	return accounts.Data[0].Id, nil
}

func logProgress(readerStdout io.Reader, readerStderr io.Reader, publishChan chan<- string, host *v3.Host, event *events.Event, errChan chan<- string, providerHandler providers.Provider) {
	// We will just logging stdout first, then stderr, ignoring all errors.
	defer close(errChan)
	scanner := bufio.NewScanner(readerStdout)
	for scanner.Scan() {
		msg := scanner.Text()
		logger.WithFields(logrus.Fields{
			"resourceId: ": event.ResourceID,
		}).Infof("stdout: %s", msg)
		transitionMsg := filterDockerMessage(msg, host, errChan, providerHandler, false)
		if transitionMsg != "" {
			publishChan <- transitionMsg
		}
	}
	scanner = bufio.NewScanner(readerStderr)
	for scanner.Scan() {
		msg := scanner.Text()
		logger.WithFields(logrus.Fields{
			"resourceId": event.ResourceID,
		}).Infof("stderr: %s", msg)
		filterDockerMessage(msg, host, errChan, providerHandler, true)
	}
}

func filterDockerMessage(msg string, host *v3.Host, errChan chan<- string, providerHandler providers.Provider, errMsg bool) string {
	if strings.Contains(msg, errorCreatingMachine) || errMsg {
		errChan <- providerHandler.HandleError(strings.Replace(msg, errorCreatingMachine, "", 1))
		return ""
	}
	if strings.Contains(msg, host.ExternalId) || strings.Contains(msg, host.Hostname) {
		return ""
	}
	return msg
}

func startReturnOutput(command *exec.Cmd) (io.Reader, io.Reader, error) {
	readerStdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	readerStderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	err = command.Start()
	if err != nil {

		defer readerStdout.Close()
		defer readerStderr.Close()
		return nil, nil, err
	}
	return readerStdout, readerStderr, nil
}

func buildCreateCommand(host *v3.Host, hostDir string, driver string) (*exec.Cmd, error) {
	cmdArgs, err := buildMachineCreateCmd(host, driver)
	if err != nil {
		return nil, err
	}

	command := buildCommand(hostDir, cmdArgs)
	return command, nil
}

func buildMachineCreateCmd(host *v3.Host, driver string) ([]string, error) {
	sDriver := strings.ToLower(driver)
	cmd := []string{"create", "-d", sDriver}

	cmd = append(cmd, buildEngineOpts("--engine-install-url", []string{host.EngineInstallUrl})...)
	cmd = append(cmd, buildEngineOpts("--engine-opt", mapToSlice(host.EngineOpt))...)
	cmd = append(cmd, buildEngineOpts("--engine-env", mapToSlice(host.EngineEnv))...)
	cmd = append(cmd, buildEngineOpts("--engine-insecure-registry", host.EngineInsecureRegistry)...)
	cmd = append(cmd, buildEngineOpts("--engine-label", mapToSlice(host.EngineLabel))...)
	cmd = append(cmd, buildEngineOpts("--engine-registry-mirror", host.EngineRegistryMirror)...)
	cmd = append(cmd, buildEngineOpts("--engine-storage-driver", []string{host.EngineStorageDriver})...)

	// Grab the reflected Value of XyzConfig (i.e. DigitaloceanConfig) based on the machine driver
	driverConfig := host.Data["fields"].(map[string]interface{})[driver+"Config"]
	if driverConfig == nil {
		return nil, fmt.Errorf("%vConfig does not exist on Machine %v", host.Driver, host.Id)
	}
	configFields := []string{}
	for k := range driverConfig.(map[string]interface{}) {
		configFields = append(configFields, k)
	}
	sort.Strings(configFields)
	driverMapConfig := driverConfig.(map[string]interface{})
	for _, nameConfigField := range configFields {
		// We are ignoring the Resource Field as we don't need it.
		if nameConfigField == "Resource" {
			continue
		}

		// This converts all field name of ParameterName to --<driver name>-parameter-name
		// i.e. AccessToken parameter for DigitalOcean driver becomes --digitalocean-access-token
		dmField := "--" + sDriver + "-" + strings.ToLower(regExHyphen.ReplaceAllString(nameConfigField, "${1}-${2}"))

		// For now, we only support bool and string.  Will add more as required.
		switch f := driverMapConfig[nameConfigField].(type) {
		case bool:
			// dm only accepts field or field=true if value=true
			if f {
				cmd = append(cmd, dmField)
			}
		case string:
			if f != "" {
				cmd = append(cmd, dmField, f)
			}
		case []string:
			for _, q := range f {
				cmd = append(cmd, dmField, q)
			}
		case []interface{}:
			for _, q := range f {
				cmd = append(cmd, dmField, fmt.Sprintf("%v", q))
			}
		case nil:
		default:
			return nil, fmt.Errorf("Unsupported type: %v", reflect.TypeOf(f))
		}

	}

	cmd = append(cmd, host.Hostname)
	logger.Infof("Cmd slice: %v", cmd)
	return cmd, nil
}

func mapToSlice(m map[string]interface{}) []string {
	ret := []string{}
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func buildEngineOpts(name string, values []string) []string {
	opts := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		opts = append(opts, name, value)
	}
	return opts
}

func createdStamp(base string, host *v3.Host) string {
	return filepath.Join(base, "machines", host.Hostname, createdFile)
}

func touchCreatedStamp(base string, machine *v3.Host) error {
	f, err := os.Create(createdStamp(base, machine))
	if err != nil {
		return err
	}
	f.Close()
	return nil
}

func createContainer(registrationURL string, host *v3.Host,
	dockerClient *client.Client, imageRepo, imageTag string) (string, error) {
	containerCmd := []string{registrationURL}
	containerConfig := buildContainerConfig(containerCmd, host, imageRepo, imageTag)
	hostConfig := buildHostConfig()

	resp, err := dockerClient.ContainerCreate(context.Background(), containerConfig, hostConfig, nil, bootstrapContName)
	if err != nil {
		return "", errors.Wrap(err, "failed to create bootstrap container")
	}
	return resp.ID, nil
}

func buildHostConfig() *container.HostConfig {
	bindConfig := []string{
		"/var/run/docker.sock:/var/run/docker.sock",
		"/var/lib/rancher:/var/lib/rancher",
	}
	hostConfig := &container.HostConfig{
		Privileged: true,
		Binds:      bindConfig,
		AutoRemove: true,
	}
	return hostConfig
}

func buildContainerConfig(containerCmd []string, host *v3.Host, imgRepo, imgTag string) *container.Config {
	image := imgRepo + ":" + imgTag

	volConfig := map[string]struct{}{
		"/var/run/docker.sock": {},
		"/var/lib/rancher":     {},
	}
	envVars := []string{"CATTLE_PHYSICAL_HOST_UUID=" + host.Uuid}
	labelVars := []string{}
	for key, value := range host.Labels {
		label := ""
		switch value.(type) {
		case string:
			label = value.(string)
		default:
			continue
		}
		labelPair := key + "=" + label
		labelVars = append(labelVars, labelPair)
	}
	if len(labelVars) > 0 {
		labelVarsString := strings.Join(labelVars, "&")
		labelVarsString = "CATTLE_HOST_LABELS=" + labelVarsString
		envVars = append(envVars, labelVarsString)
	}
	config := &container.Config{
		AttachStdin: true,
		Tty:         true,
		Image:       image,
		Volumes:     volConfig,
		Cmd:         containerCmd,
		Env:         envVars,
	}
	return config
}

func pullImage(dockerClient *client.Client, imageRepo, imageTag string) error {
	logger.Printf("pulling %v:%v image.", imageRepo, imageTag)
	reader, err := dockerClient.ImagePull(context.Background(), fmt.Sprintf("%s:%s", imageRepo, imageTag), types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	return nil
}

var getRegistrationURLAndImage = func(clusterId string, apiClient *v3.RancherClient) (string, string, string, error) {
	cluster, err := apiClient.Cluster.ById(clusterId)
	if err != nil {
		return "", "", "", err
	}

	if cluster.RegistrationToken == nil {
		return "", "", "", errors.New("Failed to find registration token")
	}

	regURL := cluster.RegistrationToken.RegistrationUrl
	repo, tag, err := parseImage(cluster.RegistrationToken.Image)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid Image format in token %s", cluster.RegistrationToken.Image)
	}

	regURL = tweakRegistrationURL(regURL)

	return regURL, repo, tag, nil
}

func tweakRegistrationURL(regURL string) string {
	// We do this to accomodate end-to-end workflow in our local development environments.
	// Containers running in a vm won't be able to reach an api running on "localhost"
	// because typically that localhost is referring to the real computer, not the vm.
	localHostReplace := os.Getenv("CATTLE_AGENT_LOCALHOST_REPLACE")
	if localHostReplace == "" {
		return regURL
	}

	regURL = strings.Replace(regURL, "localhost", localHostReplace, 1)
	return regURL
}

type tlsConnectionConfig struct {
	endpoint string
	cert     string
	key      string
	caCert   string
}

// GetDockerClient Returns a TLS-enabled docker client for the specified machine.
func GetDockerClient(machineDir string, machineName string) (*client.Client, error) {
	conf, err := getConnectionConfig(machineDir, machineName)
	if err != nil {
		return nil, fmt.Errorf("Error getting connection config: %v", err)
	}
	return newTLSDockerClient(conf)
}

func newTLSDockerClient(conf *tlsConnectionConfig) (*client.Client, error) {
	options := tlsconfig.Options{
		CAFile:             conf.caCert,
		CertFile:           conf.cert,
		KeyFile:            conf.key,
		InsecureSkipVerify: os.Getenv("DOCKER_TLS_VERIFY") == "",
	}
	tlsc, err := tlsconfig.Client(options)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsc,
		},
	}

	host := conf.endpoint
	version := defaultVersion

	cli, err := client.NewClient(host, version, httpClient, nil)
	if err != nil {
		return cli, err
	}
	return cli, nil
}

func getConnectionConfig(machineDir string, machineName string) (*tlsConnectionConfig, error) {
	command := buildCommand(machineDir, []string{"config", machineName})
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	args := string(bytes.TrimSpace(output))

	connConfig, err := parseConnectionArgs(args)
	if err != nil {
		return nil, err
	}

	return connConfig, nil
}

func parseConnectionArgs(args string) (*tlsConnectionConfig, error) {
	// Extract the -H (host) parameter
	endpointMatches := endpointRegEx.FindAllString(args, -1)
	if len(endpointMatches) != 1 {
		return nil, fmt.Errorf(parseMessage, args)
	}
	endpointKV := strings.Split(endpointMatches[0], "=")
	if len(endpointKV) != 2 {
		return nil, fmt.Errorf(parseMessage, args)
	}
	endpoint := strings.Replace(endpointKV[1], "\"", "", -1)
	config := &tlsConnectionConfig{endpoint: endpoint}
	args = endpointRegEx.ReplaceAllString(args, "")

	// Extract the tls args: tlscacert tlscert tlskey
	whitespaceSplit := regexp.MustCompile("\\w*--")
	tlsArgs := whitespaceSplit.Split(args, -1)
	for _, arg := range tlsArgs {
		kv := strings.Split(arg, "=")
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			val := strings.Trim(strings.TrimSpace(kv[1]), "\" ")
			switch key {
			case "tlscacert":
				config.caCert = val
			case "tlscert":
				config.cert = val
			case "tlskey":
				config.key = val
			}
		}
	}

	return config, nil
}

func collectImageNames(accountID string, apiClient *v3.RancherClient) ([]string, error) {
	images := []string{}
	data, err := apiClient.Service.List(&v3.ListOpts{
		Filters: map[string]interface{}{
			"system":       true,
			"accountId":    accountID,
			"removed_null": true,
		},
	})
	if err != nil {
		return nil, err
	}

	for _, service := range data.Data {
		if service.LaunchConfig == nil {
			continue
		}
		images = append(images, strings.TrimPrefix(service.LaunchConfig.ImageUuid, "docker:"))
		for _, service := range service.SecondaryLaunchConfigs {
			images = append(images, strings.TrimPrefix(service.ImageUuid, "docker:"))
		}
	}

	return images, nil
}

func parseImage(image string) (string, string, error) {
	ref, err := reference.Parse(image)
	if err != nil {
		return "", "", err
	}
	repo, tag := "", ""
	if named, ok := ref.(reference.Named); ok {
		repo = named.Name()
	}
	if tagged, ok := ref.(reference.Tagged); ok {
		tag = tagged.Tag()
	}
	return repo, tag, nil
}

func touchBootstrappedStamp(base string, host *v3.Host) error {
	f, err := os.Create(createdStamp(base, host))
	if err != nil {
		return err
	}
	f.Close()
	return nil
}
