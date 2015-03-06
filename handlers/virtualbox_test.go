package handlers

import (
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMachineHandlers(t *testing.T) {
	test_vbox := os.Getenv("TEST_VIRTUALBOX")
	if !strings.EqualFold(test_vbox, "true") {
		t.Log("Skipping virtualbox test.")
		return
	}
	setupVB()

	resourceId := "test-" + strconv.FormatInt(time.Now().Unix(), 10)
	event := &events.Event{
		ResourceId: resourceId,
		Id:         "event-id",
		ReplyTo:    "reply-to-id",
	}
	mockApiClient := &client.RancherClient{}

	err := CreateMachine(event, mockApiClient)
	if err != nil {
		t.Fatal(err)
	}

	// Idempotent check. Should rerun and reply without error
	err = CreateMachine(event, mockApiClient)
	if err != nil {
		t.Fatal(err)
	}

	// and test activating that machine
	err = ActivateMachine(event, mockApiClient)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	// Idempotent check. Should rerun and reply without error
	err = ActivateMachine(event, mockApiClient)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	// and test purging that machine
	err = PurgeMachine(event, mockApiClient)
	if err != nil {
		t.Fatal(err)
	}

	err = PurgeMachine(event, mockApiClient)
	if err != nil {
		t.Fatal(err)
	}
}

func setupVB() {
	machine := &client.Machine{
		VirtualboxConfig: client.VirtualboxConfig{
			DiskSize: "40000",
			Memory:   "2048",
		},
		Kind:   "machine",
		Driver: "VirtualBox",
	}

	getMachine = func(id string, apiClient *client.RancherClient) (*client.Machine, error) {
		machine.Id = id
		machine.Name = "name-" + id
		machine.ExternalId = "ext-" + id
		return machine, nil
	}

	getRegistrationUrl = func(accountId string, apiClient *client.RancherClient) (string, error) {
		return "http://1.2.3.4/v1", nil
	}

	publishReply = func(reply *client.Publish, apiClient *client.RancherClient) error { return nil }

	doMachineUpdate = func(current *client.Machine, machineUpdates *client.Machine,
		apiClient *client.RancherClient) error {
		if machineUpdates.Data != nil {
			machine.Data = machineUpdates.Data
		}
		return nil
	}
}
