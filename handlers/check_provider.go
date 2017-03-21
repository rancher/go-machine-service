package handlers

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/event-subscriber/events"
	client "github.com/rancher/go-rancher/v2"
)

func CheckProvider(event *events.Event, apiClient *client.RancherClient) error {
	err := checkProvider(event, apiClient)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"eventId":    event.ID,
			"resourceId": event.ResourceID,
		}).Errorf("Failed to check provider: %v", err)
	}
	return publishReply(newReply(event), apiClient)
}

func checkProvider(event *events.Event, apiClient *client.RancherClient) error {
	hosts, err := apiClient.Host.List(&client.ListOpts{
		Filters: map[string]interface{}{
			"agentId": event.ResourceID,
		},
	})
	if len(hosts.Data) != 1 || hosts.Data[0].PhysicalHostId == "" {
		return nil
	}

	host := hosts.Data[0]
	event.ResourceID = host.PhysicalHostId

	machine, machineDir, err := preEvent(event, apiClient)
	if err != nil || machine == nil {
		return err
	}

	state, err := getState(machineDir, machine)
	if err != nil {
		return err
	}

	if state == "Error" {
		logrus.WithFields(logrus.Fields{
			"hostId": host.Id,
		}).Info("Deleting host")
		_, err := apiClient.ExternalHostEvent.Create(&client.ExternalHostEvent{
			HostId:     host.Id,
			DeleteHost: true,
			EventType:  "host.evacuate",
		})
		return err
	}

	return nil
}
