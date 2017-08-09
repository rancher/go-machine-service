package handlers

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/patrickmn/go-cache"
	"github.com/rancher/event-subscriber/events"
	client "github.com/rancher/go-rancher/v3"
	"os"
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

	host, hostDir, err := getHostAndHostDir(event, apiClient)
	if err != nil || host == nil {
		return err
	}
	err = restoreMachineDir(host, hostDir)
	if err != nil {
		return err
	}
	defer os.RemoveAll(hostDir)

	mExists, err := machineExists(hostDir, host.Hostname)
	if err != nil {
		return err
	}

	if mExists {
		if err := deleteMachine(hostDir, host); err != nil {
			return err
		}
	}

	removeCache.Add(event.ResourceID, true, cache.DefaultExpiration)

	logger.WithFields(logrus.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": host.Uuid,
		"machineDir":        hostDir,
	}).Info("Machine purged")

	defer os.RemoveAll(hostDir)

	return publishReply(newReply(event), apiClient)
}
