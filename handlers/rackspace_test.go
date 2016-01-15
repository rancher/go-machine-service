package handlers

import (
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestRackspace(t *testing.T) {
	username := os.Getenv("RACKSPACE_USERNAME")
	region := os.Getenv("RACKSPACE_REGION")
	apiKey := os.Getenv("RACKSPACE_APIKEY")
	if username == "" || region == "" || apiKey == "" {
		t.Log("Skipping RACKSPACE test.")
		return
	}
	setupRackspace(username, region, apiKey)

	resourceId := "RK-" + strconv.FormatInt(time.Now().Unix(), 10)
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

func setupRackspace(username, region, apiKey string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	data["fields"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["rackspaceConfig"]= make(map[string]interface{})
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["Username"]      = username
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["Region"]        = region
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["ApiKey"]        = apiKey
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["EndpointType"]  = "publicURL"
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["ImageId"]       = "598a4282-f14b-4e50-af4c-b3e52749d9f9"
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["FlavorId"]      = "general1-1"
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["SshUser"]       = "root"
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["SshPort"]       = "22"
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["DockerInstall"] = "true"

	machine := &client.Machine{
		Data:   data,
		Kind:   "machine",
		Driver: "rackspace",
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
