package handlers

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/patrickmn/go-cache"
	"github.com/rancher/event-subscriber/events"
	client "github.com/rancher/go-rancher/v2"
)

var removeCache = cache.New(5*time.Minute, 30*time.Second)

func PurgeMachine(event *events.Event, apiClient *client.RancherClient) error {
	logger.WithFields(logrus.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
	}).Info("Purging Machine")

	if _, ok := removeCache.Get(event.ResourceID); ok {
		logger.WithFields(logrus.Fields{
			"resourceId": event.ResourceID,
			"eventId":    event.ID,
		}).Info("Machine already purged")
		return publishReply(newReply(event), apiClient)
	}

	machine, machineDirs, err := preEvent(event, apiClient)
	if err != nil || machine == nil {
		return err
	}
	defer removeMachineDir(machineDirs.jailDir)

	mExists, err := machineExists(machineDirs.jailDir, machine.Name)
	if err != nil {
		return err
	}

	if mExists {
		if err := deleteMachine(machineDirs.jailDir, machine); err != nil {
			return err
		}
	}

	removeCache.Add(event.ResourceID, true, cache.DefaultExpiration)

	logger.WithFields(logrus.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": machine.ExternalId,
		"machineDir":        machineDirs.jailDir,
	}).Info("Machine purged")

	removeMachineDir(machineDirs.jailDir)

	return publishReply(newReply(event), apiClient)
}
