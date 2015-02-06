package handlers

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-machine-service/utils"
	"github.com/rancherio/go-rancher/client"
	"io"
	"log"
	"os/exec"
	"strings"
)

const bootstrapContName = "rancher-agent-bootstrap"
const agentContName = "rancher-agent"

func CreateMachine(event *events.Event, replyHandler events.ReplyEventHandler,
	apiClient *client.RancherClient) error {
	log.Printf("Entering CreateMachine. ResourceId: %v. Event: %v.", event.ResourceId, event)

	physHost, err := apiClient.MachineHost.ById(event.ResourceId)
	if err != nil {
		return handleByIdError(err, event, replyHandler)
	}

	name := convertToName(physHost.ExternalId)

	// Be idempotent. Check if machine exists.
	cmd := exec.Command(utils.MachineCmd, "inspect", name)
	err = cmd.Run()
	if err == nil {
		replyEvent := events.NewReplyEvent(event.ReplyTo, event.Id)
		replyHandler(replyEvent)
		return nil
	}

	cmdWithArgs, err := buildMachineCreateCmd(name, physHost)
	if err != nil {
		return err
	}
	cmd = exec.Command(utils.MachineCmd, cmdWithArgs...)

	r, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			log.Printf("%s \n", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Error while reading machine create output. Error: %v. Ignoring and continuing.", err)
		}
	}(r)
	err = cmd.Wait()
	if err != nil {
		return err
	}

	replyEvent := events.NewReplyEvent(event.ReplyTo, event.Id)
	replyHandler(replyEvent)

	log.Printf("Done creating machine. ResourceId: %v. ExternalId: %v.", event.ResourceId, physHost.ExternalId)
	return nil
}

func ActivateMachine(event *events.Event, replyHandler events.ReplyEventHandler,
	apiClient *client.RancherClient) error {
	log.Printf("Entering ActivateMachine. ResourceId: %v. Event: %v.", event.ResourceId, event)

	physHost, err := apiClient.MachineHost.ById(event.ResourceId)
	if err != nil {
		return handleByIdError(err, event, replyHandler)
	}

	name := convertToName(physHost.ExternalId)

	client, err := utils.GetDockerClient(name)
	if err != nil {
		return err
	}

	// Be idempotent. Check if agent container(s) exist
	// TODO This check might be weak in the long run, but should suffice for now.
	containers, err := utils.FindContainersByNames(client, bootstrapContName, agentContName)
	if err != nil {
		return err
	}
	if len(containers) > 0 {
		replyEvent := events.NewReplyEvent(event.ReplyTo, event.Id)
		replyHandler(replyEvent)
		return nil
	}

	rancherUrl := utils.GetRancherUrl(true)
	if rancherUrl == "" {
		return errors.New("Couldn't find Rancher server URL. Can't start agent.")
	}

	imgRepo, imgTag := utils.GetRancherAgentImage()
	imageOptions := docker.PullImageOptions{
		Repository: imgRepo,
		Tag:        imgTag,
	}
	imageAuth := docker.AuthConfiguration{}
	log.Printf("Pulling %v:%v image.", imgRepo, imgTag)
	client.PullImage(imageOptions, imageAuth)

	// We are constructing a create command that looks like this:
	// docker create -it -v /var/run/docker.sock:/var/run/docker.sock --privileged \
	// rancher/agent:latest --name=tmp-rancher-agent <cattle url>

	volConfig := map[string]struct{}{"/var/run/docker.sock": {}}
	cmd := []string{rancherUrl}
	envVars := []string{"CATTLE_PHYSICAL_HOST_UUID=" + physHost.ExternalId}
	config := docker.Config{
		AttachStdin: true,
		Tty:         true,
		Image:       imgRepo + ":" + imgTag,
		Volumes:     volConfig,
		Cmd:         cmd,
		Env:         envVars,
	}

	bindConfig := []string{"/var/run/docker.sock:/var/run/docker.sock"}
	hostConfig := docker.HostConfig{
		Privileged: true,
		Binds:      bindConfig,
	}

	opts := docker.CreateContainerOptions{
		Name:       bootstrapContName,
		Config:     &config,
		HostConfig: &hostConfig}

	container, err := client.CreateContainer(opts)
	if err != nil {
		return err
	}
	log.Printf("Container created: %+v", container)

	err = client.StartContainer(container.ID, nil)
	if err != nil {
		return err
	}

	replyEvent := events.NewReplyEvent(event.ReplyTo, event.Id)
	replyHandler(replyEvent)

	log.Printf("Rancher-agent started. ResourceId: %v. ExternalId: %v Container id: %v.",
		event.ResourceId, physHost.ExternalId, container.ID)

	return nil
}

func PurgeMachine(event *events.Event, replyHandler events.ReplyEventHandler,
	apiClient *client.RancherClient) error {
	log.Printf("Entering PurgeMachine. ResourceId: %v. Event: %v.", event.ResourceId, event)

	physHost, err := apiClient.MachineHost.ById(event.ResourceId)
	if err != nil {
		return handleByIdError(err, event, replyHandler)
	}
	name := convertToName(physHost.ExternalId)

	// Be idempotent. Check if machine is already gone.
	machineExists, err := machineExists(name)
	if err != nil {
		return err
	}
	if !machineExists {
		replyEvent := events.NewReplyEvent(event.ReplyTo, event.Id)
		replyHandler(replyEvent)
		return nil
	}

	cmd := exec.Command(utils.MachineCmd, "rm", "-f", name)
	err = cmd.Start()
	if err != nil {
		return nil
	}

	err = cmd.Wait()
	if err != nil {
		return nil
	}

	replyEvent := events.NewReplyEvent(event.ReplyTo, event.Id)
	replyHandler(replyEvent)

	log.Printf("Done purging machine. ResourceId: %v. ExternalId: %v.", event.ResourceId, physHost.ExternalId)

	return nil
}

func convertToName(externalId string) string {
	// This is a small hack because a UUID is too long for virtualbox, but a UUID without dashes is not
	// TODO Consider doing this conditionally just for vbox
	return strings.Replace(externalId, "-", "", -1)
}

func machineExists(name string) (bool, error) {
	cmd := exec.Command(utils.MachineCmd, "ls", "-q")
	r, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}

	err = cmd.Start()
	if err != nil {
		return false, err
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		foundName := scanner.Text()
		if foundName == name {
			return true, nil
		}
	}
	if err = scanner.Err(); err != nil {
		return false, err
	}

	err = cmd.Wait()
	if err != nil {
		return false, err
	}

	return false, nil
}

func PingNoOp(event *events.Event, handler events.ReplyEventHandler, apiClient *client.RancherClient) error {
	// No-op ping handler
	return nil
}

func buildMachineCreateCmd(name string, physHost *client.MachineHost) ([]string, error) {
	// TODO Quick and dirty. Refactor to use reflection and maps
	// TODO Write a separate test for this function
	cmd := []string{"create", "-d"}

	switch strings.ToLower(physHost.Driver) {
	case "digitalocean":
		cmd = append(cmd, "digitalocean")
		if physHost.DigitaloceanConfig.Image != "" {
			cmd = append(cmd, "--digitalocean-image", physHost.DigitaloceanConfig.Image)
		}
		if physHost.DigitaloceanConfig.Size != "" {
			cmd = append(cmd, "--digitalocean-size", physHost.DigitaloceanConfig.Size)
		}
		if physHost.DigitaloceanConfig.Region != "" {
			cmd = append(cmd, "--digitalocean-region", physHost.DigitaloceanConfig.Region)
		}
		if physHost.DigitaloceanConfig.AccessToken != "" {
			cmd = append(cmd, "--digitalocean-access-token", physHost.DigitaloceanConfig.AccessToken)
		}
	case "virtualbox":
		cmd = append(cmd, "virtualbox")
		if physHost.VirtualboxConfig.Boot2dockerUrl != "" {
			cmd = append(cmd, "--virtualbox-boot2docker-url", physHost.VirtualboxConfig.Boot2dockerUrl)
		}
		if physHost.VirtualboxConfig.DiskSize != "" {
			cmd = append(cmd, "--virtualbox-disk-size", physHost.VirtualboxConfig.DiskSize)
		}
		if physHost.VirtualboxConfig.Memory != "" {
			cmd = append(cmd, "--virtualbox-memory", physHost.VirtualboxConfig.Memory)
		}
	default:
		return nil, fmt.Errorf("Unrecognize PhysicalHost.Kind: %v", physHost.Kind)
	}

	cmd = append(cmd, name)

	log.Printf("Cmd slice: %v", cmd)
	return cmd, nil
}

func handleByIdError(err error, event *events.Event, replyHandler events.ReplyEventHandler) error {
	apiError, ok := err.(*client.ApiError)
	if !ok || apiError.StatusCode != 404 {
		return err
	}
	// 404 Indicates this is a physicalHost but not a machineHost. Just reply.
	replyEvent := events.NewReplyEvent(event.ReplyTo, event.Id)
	replyHandler(replyEvent)
	return nil
}
