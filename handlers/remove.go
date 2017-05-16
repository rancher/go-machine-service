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

	machine, machineDir, err := preEvent(event, apiClient)
	if err != nil || machine == nil {
		return err
	}
	defer removeMachineDir(machineDir)

	mExists, err := machineExists(machineDir, machine.Name)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"resourceId":        event.ResourceID,
			"machineExternalId": machine.ExternalId,
			"machineDir":        machineDir,
		}).Warnf("Error getting machine: %s", err)
		logger.WithFields(logrus.Fields{
			"resourceId":        event.ResourceID,
			"machineExternalId": machine.ExternalId,
			"machineDir":        machineDir,
		}).Warn("Assuming machine no longer exists")
	}

	if mExists {
		if err := deleteMachine(machineDir, machine); err != nil {
			return err
		}
	}

	removeCache.Add(event.ResourceID, true, cache.DefaultExpiration)

	logger.WithFields(logrus.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": machine.ExternalId,
		"machineDir":        machineDir,
	}).Info("Machine purged")

	return publishReply(newReply(event), apiClient)
}
