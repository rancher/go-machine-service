package handlers

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	bootstrapContName   = "rancher-agent-bootstrap"
	agentContName       = "rancher-agent"
	maxWait             = time.Duration(time.Second * 10)
	bootstrappedAtField = "bootstrappedAt"
	parseMessage        = "Failed to parse config: [%v]"
	BOOTSTRAPPED_FILE   = "bootstrapped"
)

var endpointRegEx = regexp.MustCompile("-H=[[:alnum:]]*[[:graph:]]*")

func ActivateMachine(event *events.Event, apiClient *client.RancherClient) (err error) {
	log.WithFields(log.Fields{
		"resourceId": event.ResourceId,
		"eventId":    event.Id,
	}).Info("Activating Machine")

	machine, err := getMachine(event.ResourceId, apiClient)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	machineDir, err := getMachineDir(machine)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	defer func() {
		if err != nil {
			cleanupResources(machineDir, machine.Name)
		}
	}()

	dataUpdates := map[string]interface{}{}
	eventDataWrapper := map[string]interface{}{"+data": dataUpdates}

	bootstrappedFilePath := filepath.Join(machineDir, "machines", machine.Name, BOOTSTRAPPED_FILE)

	// Idempotency. If the resource has the bootstrapped file, then it has been bootstrapped.
	if _, err := os.Stat(bootstrappedFilePath); !os.IsNotExist(err) {
		if data, err := ioutil.ReadFile(bootstrappedFilePath); err != nil {
			return handleByIdError(err, event, apiClient)
		} else {
			dataUpdates[bootstrappedAtField] = string(data)
		}
		extractedConfig, extractionErr := getIdempotentExtractedConfig(machine, machineDir, apiClient)
		if extractionErr != nil {
			return handleByIdError(err, event, apiClient)
		}
		dataUpdates["+fields"] = map[string]interface{}{"extractedConfig": extractedConfig}
		reply := newReply(event)
		reply.Data = eventDataWrapper
		return publishReply(reply, apiClient)
	}

	// Setup republishing timer
	publishChan := make(chan string, 10)
	defer close(publishChan)
	go republishTransitioningReply(publishChan, event, apiClient)

	publishChan <- "Installing Rancher agent"

	registrationUrl, imageRepo, imageTag, err := getRegistrationUrlAndImage(machine.AccountId, apiClient)
	if err != nil {
		return err
	}

	dockerClient, err := GetDockerClient(machineDir, machine.Name)
	if err != nil {
		return err
	}

	err = pullImage(dockerClient, imageRepo, imageTag)
	if err != nil {
		return err
	}

	publishChan <- "Creating agent container"

	container, err := createContainer(registrationUrl, machine, dockerClient, imageRepo, imageTag)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"resourceId":  event.ResourceId,
		"machineId":   machine.Id,
		"containerId": container.ID,
	}).Info("Container created for machine")

	publishChan <- "Starting agent container"

	err = dockerClient.StartContainer(container.ID, nil)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"resourceId":        event.ResourceId,
		"machineExternalId": machine.ExternalId,
		"containerId":       container.ID,
	}).Info("Rancher-agent for machine started")

	t := time.Now()
	bootstrappedAt := t.Format(time.RFC3339)
	if f, err := os.OpenFile(bootstrappedFilePath, os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		return err
	} else {
		f.WriteString(bootstrappedAt)
		f.Close()
	}
	dataUpdates[bootstrappedAtField] = bootstrappedAt

	destFile, err := createExtractedConfig(event, machine)
	if err != nil {
		return err
	}

	if destFile != "" {
		publishChan <- "Saving Machine Config"
		extractedConf, err := getExtractedConfig(destFile, machine, apiClient)
		if err != nil {
			return err
		}
		dataUpdates["+fields"] = map[string]string{"extractedConfig": extractedConf}
	}

	reply := newReply(event)
	reply.Data = eventDataWrapper
	return publishReply(reply, apiClient)
}

func createContainer(registrationUrl string, machine *client.Machine,
	dockerClient *docker.Client, imageRepo, imageTag string) (*docker.Container, error) {
	containerCmd := []string{registrationUrl}
	containerConfig := buildContainerConfig(containerCmd, machine, imageRepo, imageTag)
	hostConfig := buildHostConfig()

	opts := docker.CreateContainerOptions{
		Name:       bootstrapContName,
		Config:     containerConfig,
		HostConfig: hostConfig}

	return dockerClient.CreateContainer(opts)
}

func buildHostConfig() *docker.HostConfig {
	bindConfig := []string{"/var/run/docker.sock:/var/run/docker.sock"}
	hostConfig := &docker.HostConfig{
		Privileged: true,
		Binds:      bindConfig,
	}
	return hostConfig
}

func buildContainerConfig(containerCmd []string, machine *client.Machine, imgRepo, imgTag string) *docker.Config {
	image := imgRepo + ":" + imgTag

	volConfig := map[string]struct{}{"/var/run/docker.sock": {}}
	envVars := []string{"CATTLE_PHYSICAL_HOST_UUID=" + machine.ExternalId}
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
	log.Printf("Pulling %v:%v image.", imageRepo, imageTag)
	err := dockerClient.PullImage(imageOptions, imageAuth)
	if err != nil {
		return err
	}
	return nil
}

var getRegistrationUrlAndImage = func(accountId string, apiClient *client.RancherClient) (string, string, string, error) {
	listOpts := client.NewListOpts()
	listOpts.Filters["accountId"] = accountId
	listOpts.Filters["state"] = "active"
	tokenCollection, err := apiClient.RegistrationToken.List(listOpts)
	if err != nil {
		return "", "", "", err
	}

	var token client.RegistrationToken
	if len(tokenCollection.Data) >= 1 {
		log.WithFields(log.Fields{
			"accountId": accountId,
		}).Debug("Found token for account")
		token = tokenCollection.Data[0]
	} else {
		log.WithFields(log.Fields{
			"accountId": accountId,
		}).Debug("Creating new token for account")
		createToken := &client.RegistrationToken{
			AccountId: accountId,
		}

		createToken, err = apiClient.RegistrationToken.Create(createToken)
		if err != nil {
			return "", "", "", err
		}
		createToken, err = waitForTokenToActivate(createToken, apiClient)
		if err != nil {
			return "", "", "", err
		}
		token = *createToken
	}

	regUrl, ok := token.Links["registrationUrl"]
	if !ok {
		return "", "", "", fmt.Errorf("No registration url on token [%v] for account [%v].", token.Id, accountId)
	}

	imageParts := strings.Split(token.Image, ":")
	if len(imageParts) != 2 {
		return "", "", "", fmt.Errorf("Invalid Image format in token [%v] for account [%v]", token.Id, accountId)
	}

	regUrl = tweakRegistrationUrl(regUrl)
	return regUrl, imageParts[0], imageParts[1], nil
}

func tweakRegistrationUrl(regUrl string) string {
	// We do this to accomodate end-to-end workflow in our local development environments.
	// Containers running in a vm won't be able to reach an api running on "localhost"
	// because typically that localhost is referring to the real computer, not the vm.
	localHostReplace := os.Getenv("CATTLE_AGENT_LOCALHOST_REPLACE")
	if localHostReplace == "" {
		return regUrl
	}

	regUrl = strings.Replace(regUrl, "localhost", localHostReplace, 1)
	return regUrl
}

func waitForTokenToActivate(token *client.RegistrationToken,
	apiClient *client.RancherClient) (*client.RegistrationToken, error) {
	timeoutAt := time.Now().Add(maxWait)
	ticker := time.NewTicker(time.Millisecond * 250)
	defer ticker.Stop()
	for t := range ticker.C {
		token, err := apiClient.RegistrationToken.ById(token.Id)
		if err != nil {
			return nil, err
		}
		if token.State == "active" {
			return token, nil
		}
		if t.After(timeoutAt) {
			return nil, fmt.Errorf("Timed out waiting for token to activate.")
		}
	}
	return nil, fmt.Errorf("Couldn't get active token.")
}

type tlsConnectionConfig struct {
	endpoint string
	cert     string
	key      string
	caCert   string
}

// Returns a TLS-enabled docker client for the specified machine.
func GetDockerClient(machineDir string, machineName string) (*docker.Client, error) {
	conf, err := getConnectionConfig(machineDir, machineName)
	if err != nil {
		return nil, err
	}

	client, err := docker.NewTLSClient(conf.endpoint, conf.cert, conf.key, conf.caCert)
	if err != nil {
		return nil, err
	}
	return client, nil
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
			val := strings.Trim(kv[1], "\" ")
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
