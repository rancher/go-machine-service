package handlers

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/api"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-machine-service/utils"
	"io"
	"log"
	"os/exec"
	"strings"
)

const bootstrapContName = "rancher-agent-bootstrap"
const agentContName = "rancher-agent"

func CreateMachine(event *events.Event, replyHandler events.ReplyEventHandler, apiClient api.Client) error {
	log.Printf("Creating machine. ResourceId: %v. Event: %v.", event.ResourceId, event)

	physHost, err := apiClient.GetPhysicalHost(event.ResourceId)
	if err != nil {
		return err
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

func ActivateMachine(event *events.Event, replyHandler events.ReplyEventHandler, apiClient api.Client) error {
	log.Printf("Activating machine. ResourceId: %v. Event: %v.", event.ResourceId, event)

	physHost, err := apiClient.GetPhysicalHost(event.ResourceId)
	if err != nil {
		return err
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
		return errors.New("Couldn't find Rancher server URL. Can't start agent. Returning.")
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
	// TODO Calmn down on this log statement
	log.Printf("Container created: %v", container)

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

func PurgeMachine(event *events.Event, replyHandler events.ReplyEventHandler, apiClient api.Client) error {
	log.Printf("Purging machine. ResourceId: %v. Event: %v.", event.ResourceId, event)

	physHost, err := apiClient.GetPhysicalHost(event.ResourceId)
	if err != nil {
		return err
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

func PingNoOp(event *events.Event, handler events.ReplyEventHandler, apiClient api.Client) error {
	// No-op ping handler
	return nil
}

func buildMachineCreateCmd(name string, physHost *api.PhysicalHost) ([]string, error) {
	// TODO Quick and dirty. Refactor to use reflection and maps
	cmd := []string{"create", "-d"}

	switch physHost.Kind {
	case "digitalOceanHost":
		cmd = append(cmd, "digitalocean")
		if physHost.Image != "" {
			cmd = append(cmd, "--digitalocean-image", physHost.Image)
		}
		if physHost.Size != "" {
			cmd = append(cmd, "--digitalocean-size", physHost.Size)
		}
		if physHost.Region != "" {
			cmd = append(cmd, "--digitalocean-region", physHost.Region)
		}
		if physHost.AccessToken != "" {
			cmd = append(cmd, "--digitalocean-access-token", physHost.AccessToken)
		}
	case "googleHost":
		cmd = append(cmd, "google")
		if physHost.MachineType != "" {
			cmd = append(cmd, "--google-machine-type", physHost.MachineType)
		}
		if physHost.Project != "" {
			cmd = append(cmd, "--google-project", physHost.Project)
		}
		if physHost.Username != "" {
			cmd = append(cmd, "--google-username", physHost.Username)
		}
		if physHost.Zone != "" {
			cmd = append(cmd, "--google-zone", physHost.Zone)
		}
	case "virtualBoxHost":
		cmd = append(cmd, "virtualbox")
		if physHost.Boot2dockerUrl != "" {
			cmd = append(cmd, "--virtualbox-boot2docker-url", physHost.Boot2dockerUrl)
		}
		if physHost.DiskSize != "" {
			cmd = append(cmd, "--virtualbox-disk-size", physHost.DiskSize)
		}
		if physHost.Memory != "" {
			cmd = append(cmd, "--virtualbox-memory", physHost.Memory)
		}
	case "amazonEc2Host":
		cmd = append(cmd, "amazonec2")
		if physHost.AccessKey != "" {
			cmd = append(cmd, "--amazonec2-access-key", physHost.AccessKey)
		}
		if physHost.Ami != "" {
			cmd = append(cmd, "--amazonec2-ami", physHost.Ami)
		}
		if physHost.InstanceType != "" {
			cmd = append(cmd, " --amazonec2-instance-type", physHost.InstanceType)
		}
		if physHost.Region != "" {
			cmd = append(cmd, "--amazonec2-region", physHost.Region)
		}
		if physHost.RootSize != "" {
			cmd = append(cmd, "--amazonec2-root-size", physHost.RootSize)
		}
		if physHost.SecretKey != "" {
			cmd = append(cmd, "--amazonec2-secret-key", physHost.SecretKey)
		}
		if physHost.SessionToken != "" {
			cmd = append(cmd, "--amazonec2-session-token", physHost.SessionToken)
		}
		if physHost.SessionToken != "" {
			cmd = append(cmd, "--amazonec2-subnet-id", physHost.SessionToken)
		}
		if physHost.VpcId != "" {
			cmd = append(cmd, "--amazonec2-vpc-id", physHost.VpcId)
		}
		if physHost.Zone != "" {
			cmd = append(cmd, "--amazonec2-zone", physHost.Zone)
		}
	default:
		return nil, fmt.Errorf("Unrecognize PhysicalHost.Kind: %v", physHost.Kind)
	}

	cmd = append(cmd, name)

	log.Printf("Cmd slice: %v", cmd)
	return cmd, nil
}
