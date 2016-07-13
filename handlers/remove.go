package handlers

import (
	"bufio"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/event-subscriber/events"
	"github.com/rancher/go-rancher/client"
	"os"
	"path/filepath"
)

func PurgeMachine(event *events.Event, apiClient *client.RancherClient) error {
	log.WithFields(log.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
	}).Info("Purging Machine")

	machine, err := getMachine(event.ResourceID, apiClient)
	if err != nil {
		return err
	}
	if machine == nil {
		return notAMachineReply(event, apiClient)
	}

	baseMachineDir, err := getBaseMachineDir(machine.ExternalId)
	if err != nil {
		return fmt.Errorf("Unable to determine base machine directory. Cannot purge machine %v Nothing to do. Error: %v", machine.Name, err)
	}

	// If this dir doesn't exist, we have nothing to do.
	dExists, err := dirExists(baseMachineDir)
	if err != nil {
		return fmt.Errorf("Unable to determine if base machine directory exists. Cannot purge machine %v. Error: %v", machine.Name, err)
	}

	if !dExists {
		if ignoreExtractedConfig(machine.Driver) {
			reply := newReply(event)
			return publishReply(reply, apiClient)
		}

		err := reinitFromExtractedConfig(machine, filepath.Dir(baseMachineDir))
		if err != nil {
			if err != errNoExtractedConfig {
				return err
			}
		}
	}

	machineDir, err := getMachineDir(machine)
	if err != nil {
		return fmt.Errorf("Unable to determine machine directory. Cannot purge machine %v Nothing to do. Error: %v", machine.Name, err)
	}

	mExists, err := machineExists(machineDir, machine.Name)
	if err != nil {
		return err
	}

	if mExists {
		err := deleteMachine(machineDir, machine)
		if err != nil {
			return err
		}
	}

	err = os.RemoveAll(baseMachineDir)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": machine.ExternalId,
		"machineDir":        machineDir,
	}).Info("Machine purged")

	reply := newReply(event)
	return publishReply(reply, apiClient)
}

func deleteMachine(machineDir string, machine *client.Machine) error {
	command := buildCommand(machineDir, []string{"rm", "-f", machine.Name})
	err := command.Start()
	if err != nil {
		return err
	}

	err = command.Wait()
	if err != nil {
		return err
	}

	return nil
}

func machineExists(machineDir string, name string) (bool, error) {
	command := buildCommand(machineDir, []string{"ls", "-q"})
	r, err := command.StdoutPipe()
	if err != nil {
		return false, err
	}

	err = command.Start()
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

	err = command.Wait()
	if err != nil {
		return false, err
	}

	return false, nil
}
