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

	resourceID := "RK-" + strconv.FormatInt(time.Now().Unix(), 10)
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

	err = ActivateMachine(event, mockAPIClient)
	if err != nil {
		// Fail, not a fatal, so purge will still run.
		t.Log(err)
		t.Fail()
	}

	err = PurgeMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}
}

func setupRackspace(username, region, apiKey string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	rackspaceConfig := make(map[string]interface{})
	fields["rackspaceConfig"] = rackspaceConfig
	rackspaceConfig["username"] = username
	rackspaceConfig["region"] = region
	rackspaceConfig["apiKey"] = apiKey
	rackspaceConfig["endpointType"] = "publicURL"
	rackspaceConfig["imageId"] = "598a4282-f14b-4e50-af4c-b3e52749d9f9"
	rackspaceConfig["flavorId"] = "general1-1"
	rackspaceConfig["sshUser"] = "root"
	rackspaceConfig["sshPort"] = "22"
	rackspaceConfig["dockerInstall"] = "true"

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

	getRegistrationURLAndImage = func(accountId string, apiClient *client.RancherClient) (string, string, string, string, error) {
		return "http://1.2.3.4/v1", "rancher/agent", "v0.7.6", "", nil
	}

	publishReply = buildMockPublishReply(machine)
	publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {}
}
