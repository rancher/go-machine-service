package handlers

import (
	"bufio"
	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
	"os"
)

func PurgeMachine(event *events.Event, apiClient *client.RancherClient) error {
	log.WithFields(log.Fields{
		"ResourceId": event.ResourceId,
		"EventId":    event.Id,
	}).Info("Purging Machine")

	machine, err := getMachine(event.ResourceId, apiClient)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	machineDir, err := getMachineDir(machine)
	if err != nil {
		// No machine dir, nothing to do.
		log.WithFields(log.Fields{
			"ResourceId": event.ResourceId,
			"Err":        err,
		}).Warn("Unable to find machineDir.  Nothing to do")
		reply := newReply(event)
		return publishReply(reply, apiClient)
	}

	// Idempotency. If this dir doesn't exist, we have nothing to do.
	dExists, err := dirExists(machineDir)
	if !dExists {
		reply := newReply(event)
		return publishReply(reply, apiClient)
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

	err = os.RemoveAll(machineDir)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"ResourceId":        event.ResourceId,
		"MachineExternalId": machine.ExternalId,
		"MachineDir":        machineDir,
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

func dirExists(machineDir string) (bool, error) {
	_, err := os.Stat(machineDir)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
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
