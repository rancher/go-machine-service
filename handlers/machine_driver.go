package handlers

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/event-subscriber/events"
	"github.com/rancher/go-machine-service/dynamic"
	"github.com/rancher/go-rancher/v2"
)

func DeactivateDriver(event *events.Event, apiClient *client.RancherClient) error {
	return removeDriver(event, apiClient, false)
}

func RemoveDriver(event *events.Event, apiClient *client.RancherClient) error {
	return removeDriver(event, apiClient, true)
}

func removeDriver(event *events.Event, apiClient *client.RancherClient, delete bool) error {
	logrus.WithFields(logrus.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
		"name":       event.Name,
	}).Info("Event")

	driverInfo, err := apiClient.MachineDriver.ById(event.ResourceID)
	if err != nil {
		return err
	}

	if err := dynamic.RemoveSchemas(driverInfo.Name+"Config", apiClient); err != nil {
		return err
	}

	if driverInfo.Checksum == "" || delete {
		driver, err := getDriver(event.ResourceID, apiClient)
		if err == nil {
			logrus.Infof("Removing driver %s", driverInfo.Name)
			driver.Remove()
		}
	}

	if err := dynamic.UploadMachineSchemas(apiClient); err != nil {
		return err
	}

	reply := newReply(event)
	return publishReply(reply, apiClient)
}

func ActivateDriver(event *events.Event, apiClient *client.RancherClient) error {
	logrus.WithFields(logrus.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
		"name":       event.Name,
	}).Info("Event")

	driver, err := activate(event.ResourceID, apiClient)
	if err != nil {
		return err
	}

	version, err := dynamic.DriverSchemaVersion(apiClient)
	if err != nil {
		return err
	}

	reply := newReply(event)
	reply.Data = map[string]interface{}{
		"name":          driver.FriendlyName(),
		"defaultActive": false,
		"schemaVersion": version,
	}

	if err := dynamic.UploadMachineSchemas(apiClient, driver.FriendlyName()); err != nil {
		return err
	}

	return publishReply(reply, apiClient)
}

func getDriver(id string, apiClient *client.RancherClient) (*dynamic.Driver, error) {
	driverInfo, err := apiClient.MachineDriver.ById(id)
	if err != nil {
		return nil, err
	}

	return dynamic.NewDriver(driverInfo.Builtin, driverInfo.Name, driverInfo.Url, driverInfo.Checksum), nil
}

func activate(id string, apiClient *client.RancherClient) (*dynamic.Driver, error) {
	driver, err := getDriver(id, apiClient)
	if err != nil {
		return nil, err
	}

	if err := driver.Stage(); err != nil {
		return nil, err
	}

	opts := client.NewListOpts()
	opts.Filters["name"] = driver.FriendlyName()
	opts.Filters["state"] = "active"
	existing, err := apiClient.MachineDriver.List(opts)
	if err != nil {
		return nil, err
	}

	if len(existing.Data) > 0 {
		return nil, fmt.Errorf("An active driver name %s already exists", driver.Name())
	}

	if err := driver.Install(); err != nil {
		return nil, err
	}

	return driver, dynamic.GenerateAndUploadSchema(driver.Name())
}
