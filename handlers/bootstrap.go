package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/distribution/reference"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/rancher/event-subscriber/events"
	client "github.com/rancher/go-rancher/v2"
)

const (
	bootstrapContName = "rancher-agent-bootstrap"
	maxWait           = time.Duration(time.Second * 10)
	parseMessage      = "Failed to parse config: [%v]"
	bootStrappedFile  = "bootstrapped"
	fingerprintStart  = "CA_FINGERPRINT="
)

var endpointRegEx = regexp.MustCompile("-H=[[:alnum:]]*[[:graph:]]*")

func ActivateMachine(event *events.Event, apiClient *client.RancherClient) (err error) {
	logger.WithFields(logrus.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
	}).Info("Activating Machine")

	machine, machineDirs, err := preEvent(event, apiClient)
	if err != nil || machine == nil {
		return nil
	}
	defer removeMachineDir(machineDirs.jailDir)

	// If the resource has the bootstrapped file, then it has been bootstrapped.
	if _, err := os.Stat(bootstrappedStamp(machineDirs.fullMachinePath, machine)); err == nil {
		if err := saveMachineConfig(machineDirs.fullMachinePath, machine, apiClient); err != nil {
			return err
		}
		return publishReply(newReply(event), apiClient)
	}

	// Setup republishing timer
	publishChan := make(chan string, 10)
	defer close(publishChan)
	go republishTransitioningReply(publishChan, event, apiClient)

	publishChan <- "Installing Rancher agent"

	registrationURL, imageRepo, imageTag, fingerprint, err := getRegistrationURLAndImage(machine.AccountId, apiClient)
	if err != nil {
		return err
	}

	dockerClient, err := GetDockerClient(machineDirs.jailDir, machine.Name)
	if err != nil {
		return err
	}

	err = pullImage(dockerClient, imageRepo, imageTag)
	if err != nil {
		return err
	}

	publishChan <- "Creating agent container"

	container, err := createContainer(registrationURL, machine, dockerClient, imageRepo, imageTag, fingerprint)
	if err != nil {
		return err
	}
	logger.WithFields(logrus.Fields{
		"resourceId":  event.ResourceID,
		"machineId":   machine.Id,
		"containerId": container.ID,
	}).Info("Container created for machine")

	publishChan <- "Starting agent container"

	err = dockerClient.StartContainer(container.ID, nil)
	if err != nil {
		return err
	}

	found := false
	for i := 0; i < 30; i++ {
		containers, err := dockerClient.ListContainers(docker.ListContainersOptions{})
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
			"machineId":  machine.Id,
		}).Error("Failed to find rancher-agent container")
		return errors.New("Failed to find rancher-agent container")
	}

	go func() {
		images, err := collectImageNames(machine.AccountId, apiClient)
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
				"machineExternalId": machine.ExternalId,
				"repo":              repo,
				"tag":               tag,
			}).Info("Pulling image")
			go pullImage(dockerClient, repo, tag)
		}
	}()

	logger.WithFields(logrus.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": machine.ExternalId,
		"containerId":       container.ID,
	}).Info("Rancher-agent for machine started")

	if err := touchBootstrappedStamp(machineDirs.fullMachinePath, machine); err != nil {
		return err
	}

	if err := saveMachineConfig(machineDirs.fullMachinePath, machine, apiClient); err != nil {
		return err
	}

	return publishReply(newReply(event), apiClient)
}

func createContainer(registrationURL string, machine *client.Machine,
	dockerClient *docker.Client, imageRepo, imageTag, fingerprint string) (*docker.Container, error) {
	containerCmd := []string{registrationURL}
	containerConfig := buildContainerConfig(containerCmd, machine, imageRepo, imageTag, fingerprint)
	hostConfig := buildHostConfig()

	opts := docker.CreateContainerOptions{
		Name:       bootstrapContName,
		Config:     containerConfig,
		HostConfig: hostConfig}

	return dockerClient.CreateContainer(opts)
}

func buildHostConfig() *docker.HostConfig {
	bindConfig := []string{
		"/var/run/docker.sock:/var/run/docker.sock",
		"/var/lib/rancher:/var/lib/rancher",
	}
	hostConfig := &docker.HostConfig{
		Privileged: true,
		Binds:      bindConfig,
	}
	return hostConfig
}

func buildContainerConfig(containerCmd []string, machine *client.Machine, imgRepo, imgTag, fingerprint string) *docker.Config {
	image := imgRepo + ":" + imgTag

	volConfig := map[string]struct{}{
		"/var/run/docker.sock": {},
		"/var/lib/rancher":     {},
	}
	envVars := []string{"CATTLE_PHYSICAL_HOST_UUID=" + machine.ExternalId,
		"CATTLE_DOCKER_UUID=" + machine.ExternalId}
	labelVars := []string{}
	for key, value := range machine.Labels {
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
	if fingerprint != "" {
		envVars = append(envVars, strings.Replace(fingerprint, "\"", "", -1))
	}
	config := &docker.Config{
		AttachStdin: true,
		Tty:         true,
		Image:       image,
		Volumes:     volConfig,
		Cmd:         containerCmd,
		Env:         envVars,
	}
	return config
}

func pullImage(dockerClient *docker.Client, imageRepo, imageTag string) error {
	imageOptions := docker.PullImageOptions{
		Repository: imageRepo,
		Tag:        imageTag,
	}
	imageAuth := docker.AuthConfiguration{}
	logger.Printf("pulling %v:%v image.", imageRepo, imageTag)
	err := dockerClient.PullImage(imageOptions, imageAuth)
	if err != nil {
		return err
	}
	return nil
}

var getRegistrationURLAndImage = func(accountID string, apiClient *client.RancherClient) (string, string, string, string, error) {
	listOpts := client.NewListOpts()
	listOpts.Filters["accountId"] = accountID
	listOpts.Filters["state"] = "active"
	tokenCollection, err := apiClient.RegistrationToken.List(listOpts)
	if err != nil {
		return "", "", "", "", err
	}

	var token client.RegistrationToken
	if len(tokenCollection.Data) >= 1 {
		logger.WithFields(logrus.Fields{
			"accountId": accountID,
		}).Debug("Found token for account")
		token = tokenCollection.Data[0]
	} else {
		logger.WithFields(logrus.Fields{
			"accountId": accountID,
		}).Debug("Creating new token for account")
		createToken := &client.RegistrationToken{
			AccountId: accountID,
		}

		createToken, err = apiClient.RegistrationToken.Create(createToken)
		if err != nil {
			return "", "", "", "", err
		}
		createToken, err = waitForTokenToActivate(createToken, apiClient)
		if err != nil {
			return "", "", "", "", err
		}
		token = *createToken
	}

	regURL, ok := token.Links["registrationUrl"]
	if !ok {
		return "", "", "", "", fmt.Errorf("no registration url on token [%v] for account [%v]", token.Id, accountID)
	}

	repo, tag, err := parseImage(token.Image)
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid Image format in token [%v] for account [%v]", token.Id, accountID)
	}

	regURL = tweakRegistrationURL(regURL)

	return regURL, repo, tag, parseFingerprint(token), nil
}

func parseFingerprint(token client.RegistrationToken) string {
	for _, part := range strings.Fields(token.Command) {
		if strings.HasPrefix(part, fingerprintStart) {
			return part
		}
	}
	return ""
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

func waitForTokenToActivate(token *client.RegistrationToken,
	apiClient *client.RancherClient) (*client.RegistrationToken, error) {
	timeoutAt := time.Now().Add(maxWait)
	ticker := time.NewTicker(time.Millisecond * 250)
	defer ticker.Stop()
	tokenID := token.Id
	for t := range ticker.C {
		token, err := apiClient.RegistrationToken.ById(tokenID)
		if err != nil {
			return nil, err
		}
		if token == nil {
			return nil, fmt.Errorf("couldn't find token %v", tokenID)
		}
		if token.State == "active" {
			return token, nil
		}
		if t.After(timeoutAt) {
			return nil, fmt.Errorf("timed out waiting for token to activate")
		}
	}
	return nil, fmt.Errorf("Couldn't get active token")
}

type tlsConnectionConfig struct {
	endpoint string
	cert     string
	key      string
	caCert   string
}

// GetDockerClient Returns a TLS-enabled docker client for the specified machine.
func GetDockerClient(machineDir string, machineName string) (*docker.Client, error) {
	conf, err := getConnectionConfig(machineDir, machineName)
	if err != nil {
		return nil, fmt.Errorf("Error getting connection config: %v", err)
	}

	client, err := docker.NewTLSClient(conf.endpoint, conf.cert, conf.key, conf.caCert)
	if err != nil {
		return nil, fmt.Errorf("Error getting docker client: %v", err)
	}
	return client, nil
}

func getConnectionConfig(machineDir string, machineName string) (*tlsConnectionConfig, error) {
	command, err := buildCommand(machineDir, []string{"config", machineName})
	if err != nil {
		return nil, err
	}
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	args := string(bytes.TrimSpace(output))

	connConfig, err := parseConnectionArgs(args)
	if err != nil {
		return nil, err
	}

	if os.Getenv("DISABLE_DRIVER_JAIL") != "true" {
		// Docker reads from the local file system, it does not exec a docker-machine command
		// so give it the full path if running in jail mode
		connConfig.caCert = path.Join(machineDir, connConfig.caCert)
		connConfig.cert = path.Join(machineDir, connConfig.cert)
		connConfig.key = path.Join(machineDir, connConfig.key)
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

func collectImageNames(accountID string, apiClient *client.RancherClient) ([]string, error) {
	images := []string{}
	data, err := apiClient.Service.List(&client.ListOpts{
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

func bootstrappedStamp(base string, machine *client.Machine) string {
	return filepath.Join(base, "machines", machine.Name, bootStrappedFile)
}

func touchBootstrappedStamp(base string, machine *client.Machine) error {
	f, err := os.Create(createdStamp(base, machine))
	if err != nil {
		return err
	}
	f.Close()
	return nil
}
