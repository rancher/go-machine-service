package handlers

import (
	"bufio"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CreateMachine(event *events.Event, apiClient *client.RancherClient) error {
	log.WithFields(log.Fields{
		"ResourceId": event.ResourceId,
		"Event":      event,
	}).Info("Creating Machine")

	machine, err := getMachine(event.ResourceId, apiClient)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	// Idempotency. If the resource has the property, we're done.
	if _, ok := machine.Data[machineDirField]; ok {
		reply := newReply(event)
		return publishReply(reply, apiClient)
	}

	command, machineDir, err := buildCreateCommand(machine)
	if err != nil {
		return err
	}

	readerStdout, readerStderr, err := startReturnOutput(command)
	if err != nil {
		return err
	}

	go logProgress(event.ResourceId, readerStdout, readerStderr)

	err = command.Wait()

	if err != nil {
		return err
	}

	updates := map[string]string{machineDirField: machineDir}
	err = updateMachineData(machine, updates, apiClient)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"ResourceId":         event.ResourceId,
		"Machine ExternalId": machine.ExternalId,
	}).Info("Machine Created")

	reply := newReply(event)
	return publishReply(reply, apiClient)
}

func logProgress(resourceId string, readerStdout io.Reader, readerStderr io.Reader) {
	// We will just log stdout first, then stderr, ignoring all errors.
	scanner := bufio.NewScanner(readerStdout)
	for scanner.Scan() {
		log.WithFields(log.Fields{
			"ResourceId: ": resourceId,
		}).Infof("stdout: %s", scanner.Text())
	}

	scanner = bufio.NewScanner(readerStderr)
	for scanner.Scan() {
		log.WithFields(log.Fields{
			"ResourceId": resourceId,
		}).Infof("stderr: %s", scanner.Text())
	}
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

func buildCreateCommand(machine *client.Machine) (*exec.Cmd, string, error) {
	cmdArgs, err := buildMachineCreateCmd(machine)
	if err != nil {
		return nil, "", err
	}

	machineDir, err := buildMachineDir(machine.ExternalId)
	if err != nil {
		return nil, "", err
	}

	command := buildCommand(machineDir, cmdArgs)
	return command, machineDir, nil
}

func buildMachineDir(uuid string) (string, error) {
	cattleHome := os.Getenv("CATTLE_HOME")
	if cattleHome == "" {
		return "", fmt.Errorf("CATTLE_HOME not set. Cant create machine. Uuid: [%v].", uuid)
	}
	machineDir := filepath.Join(cattleHome, "machine", uuid)
	err := os.MkdirAll(machineDir, 0740)
	if err != nil {
		return "", err
	}
	return machineDir, err
}

func buildMachineCreateCmd(machine *client.Machine) ([]string, error) {
	// TODO Quick and dirty. Refactor to use reflection and maps
	// TODO Write a separate test for this function
	cmd := []string{"create", "-d"}

	switch strings.ToLower(machine.Driver) {
	case "digitalocean":
		cmd = append(cmd, "digitalocean")
		if machine.DigitaloceanConfig.Image != "" {
			cmd = append(cmd, "--digitalocean-image", machine.DigitaloceanConfig.Image)
		}
		if machine.DigitaloceanConfig.Size != "" {
			cmd = append(cmd, "--digitalocean-size", machine.DigitaloceanConfig.Size)
		}
		if machine.DigitaloceanConfig.Region != "" {
			cmd = append(cmd, "--digitalocean-region", machine.DigitaloceanConfig.Region)
		}
		if machine.DigitaloceanConfig.AccessToken != "" {
			cmd = append(cmd, "--digitalocean-access-token", machine.DigitaloceanConfig.AccessToken)
		}
	case "virtualbox":
		cmd = append(cmd, "virtualbox")
		if machine.VirtualboxConfig.Boot2dockerUrl != "" {
			cmd = append(cmd, "--virtualbox-boot2docker-url", machine.VirtualboxConfig.Boot2dockerUrl)
		}
		if machine.VirtualboxConfig.DiskSize != "" {
			cmd = append(cmd, "--virtualbox-disk-size", machine.VirtualboxConfig.DiskSize)
		}
		if machine.VirtualboxConfig.Memory != "" {
			cmd = append(cmd, "--virtualbox-memory", machine.VirtualboxConfig.Memory)
		}
	default:
		return nil, fmt.Errorf("Unrecognize Driver: %v", machine.Driver)
	}

	cmd = append(cmd, machine.Name)

	log.Infof("Cmd slice: %v", cmd)
	return cmd, nil
}
