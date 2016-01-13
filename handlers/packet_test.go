package handlers

import (
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestPacket(t *testing.T) {
	apiKey := os.Getenv("PACKET_API_KEY")
	projectId := os.Getenv("PACKET_PROJECT_ID")
	if apiKey == "" || projectId == "" {
		t.Log("Skipping packet test.")
		return
	}
	setupPacket(apiKey, projectId)

	resourceId := "PA-" + strconv.FormatInt(time.Now().Unix(), 10)
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

	err = ActivateMachine(event, mockApiClient)
	if err != nil {
		// Fail, not a fatal, so purge will still run.
		t.Log(err)
		t.Fail()
	}

	err = PurgeMachine(event, mockApiClient)
	if err != nil {
		t.Fatal(err)
	}
}

func setupPacket(apiKey, projectId string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	data["fields"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["packetConfig"]= make(map[string]interface{})
	data["fields"].(map[string]interface{})["packetConfig"].(map[string]interface{})["ApiKey"] =  apiKey
	data["fields"].(map[string]interface{})["packetConfig"].(map[string]interface{})["ProjectId"] =  projectId

	machine := &client.Machine{
		Data: data,
		Kind:   "machine",
		Driver: "Packet",
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
