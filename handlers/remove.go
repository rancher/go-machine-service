package handlers

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/event-subscriber/events"
	client "github.com/rancher/go-rancher/v2"
)

func PurgeMachine(event *events.Event, apiClient *client.RancherClient) error {
	logger.WithFields(logrus.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
	}).Info("Purging Machine")

	machine, machineDir, err := preEvent(event, apiClient)
	if err != nil || machine == nil {
		return err
	}
	defer removeMachineDir(machineDir)

	mExists, err := machineExists(machineDir, machine.Name)
	if err != nil {
		return err
	}

	if mExists {
		if err := deleteMachine(machineDir, machine); err != nil {
			return err
		}
	}

	logger.WithFields(logrus.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": machine.ExternalId,
		"machineDir":        machineDir,
	}).Info("Machine purged")

	removeMachineDir(machineDir)

	return publishReply(newReply(event), apiClient)
}
