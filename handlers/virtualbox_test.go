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
	testVbox := os.Getenv("TEST_VIRTUALBOX")
	if !strings.EqualFold(testVbox, "true") {
		t.Log("Skipping virtualbox test.")
		return
	}
	setupVB()

	resourceID := "test-" + strconv.FormatInt(time.Now().Unix(), 10)
	event := &events.Event{
		ResourceID: resourceID,
		ID:         "event-id",
		ReplyTo:    "reply-to-id",
	}
	mockAPIClient := &client.RancherClient{}

	err := CreateMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	// Idempotent check. Should rerun and reply without error
	err = CreateMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	// and test activating that machine
	err = ActivateMachine(event, mockAPIClient)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	// Idempotent check. Should rerun and reply without error
	err = ActivateMachine(event, mockAPIClient)
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	// and test purging that machine
	err = PurgeMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	err = PurgeMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}
}

func setupVB() {

	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	virtualboxConfig := make(map[string]interface{})
	fields["virtualboxConfig"] = virtualboxConfig
	virtualboxConfig["diskSize"] = "40000"
	virtualboxConfig["memory"] = "2048"

	machine := &client.Machine{
		Data:   data,
		Kind:   "machine",
		Driver: "virtualbox",
	}

	getMachine = func(id string, apiClient *client.RancherClient) (*client.Machine, error) {
		machine.Id = id
		machine.Name = "name-" + id
		machine.ExternalId = "ext-" + id
		return machine, nil
	}

	getRegistrationURLAndImage = func(accountId string, apiClient *client.RancherClient) (string, string, string, error) {
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

			if fields, ok := d["+fields"]; ok {
				if fieldMap, ok := fields.(map[string]interface{}); ok {
					if extractedConfig, ok := fieldMap["extractedConfig"]; ok {
						if conf, ok := extractedConfig.(string); ok {
							machine.ExtractedConfig = conf
						}
					}
				}
			}
		}
		return nil
	}
}
