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
	machine := &client.Machine{
		RackspaceConfig: &client.RackspaceConfig{
			Username:      username,
			Region:        region,
			ApiKey:        apiKey,
			EndpointType:  "publicURL",
			ImageId:       "598a4282-f14b-4e50-af4c-b3e52749d9f9",
			FlavorId:      "general1-1",
			SshUser:       "root",
			SshPort:       "22",
			DockerInstall: "true",
		},
		Kind:   "machine",
		Driver: "Rackspace",
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
