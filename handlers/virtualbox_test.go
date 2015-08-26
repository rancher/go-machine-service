package handlers

import (
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
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
		VirtualboxConfig: &client.VirtualboxConfig{
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

	getRegistrationUrlAndImage = func(accountId string, apiClient *client.RancherClient) (string, string, string, error) {
		return "http://1.2.3.4/v1", "rancher/agent", "v0.7.6", nil
	}

	publishReply = buildMockPublishReply(machine)

	publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {}
}

type mockPublishReplyFunc func(reply *client.Publish, apiClient *client.RancherClient) error

func buildMockPublishReply(machine *client.Machine) mockPublishReplyFunc {
	return func(reply *client.Publish, apiClient *client.RancherClient) error {
		if reply.Data == nil {
			return nil
		}

		if machine.Data == nil {
			machine.Data = map[string]interface{}{}
		}

		if data, ok := reply.Data["+data"]; ok {
			d := data.(map[string]interface{})
			if machineDir, mdOk := d[machineDirField]; mdOk {
				machine.Data[machineDirField] = machineDir
			}

			if bootstrap, bootOk := d[bootstrappedAtField]; bootOk {
				machine.Data[bootstrappedAtField] = bootstrap
			}
		}
		return nil
	}
}
