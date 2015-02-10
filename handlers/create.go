package handlers

import (
	"bufio"
	"fmt"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CreateMachine(event *events.Event, apiClient *client.RancherClient) error {
	log.Printf("Entering CreateMachine. ResourceId: %v. Event: %v.", event.ResourceId, event)

	physHost, err := getMachine(event.ResourceId, apiClient)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	// Idempotency. If the resource has the property, we're done.
	if _, ok := physHost.Data[machineDirField]; ok {
		reply := newReply(event)
		return publishReply(reply, apiClient)
	}

	command, machineDir, err := buildCreateCommand(physHost)
	if err != nil {
		return err
	}

	reader, err := startReturnOutput(command)
	if err != nil {
		return err
	}

	go logProgress(reader)

	err = command.Wait()
	if err != nil {
		return err
	}

	updates := map[string]string{machineDirField: machineDir}
	err = updateMachineData(physHost, updates, apiClient)
	if err != nil {
		return err
	}

	log.Printf("Done creating machine. ResourceId: %v. ExternalId: %v.",
		event.ResourceId, physHost.ExternalId)

	reply := newReply(event)
	return publishReply(reply, apiClient)
}

func logProgress(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		log.Printf("%s \n", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading output: %v. Ignoring and continuing.", err)
	}
}

func startReturnOutput(command *exec.Cmd) (io.Reader, error) {
	reader, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = command.Start()
	if err != nil {
		defer reader.Close()
		return nil, err
	}
	return reader, nil
}

func buildCreateCommand(physHost *client.MachineHost) (*exec.Cmd, string, error) {
	cmdArgs, err := buildMachineCreateCmd(physHost)
	if err != nil {
		return nil, "", err
	}

	machineDir, err := buildMachineDir(physHost.ExternalId)
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

func buildMachineCreateCmd(physHost *client.MachineHost) ([]string, error) {
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
		return nil, fmt.Errorf("Unrecognize Driver: %v", physHost.Driver)
	}

	cmd = append(cmd, physHost.Name)

	log.Printf("Cmd slice: %v", cmd)
	return cmd, nil
}
