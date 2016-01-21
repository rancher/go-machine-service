package handlers

import (
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestDigitalOcean(t *testing.T) {
	access_token := os.Getenv("DIGITALOCEAN_KEY")
	if access_token == "" {
		t.Log("Skipping Digital Ocean test.")
		return
	}
	setupDO(access_token)

	resourceId := "DO-" + strconv.FormatInt(time.Now().Unix(), 10)
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

func setupDO(access_token string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	digitaloceanConfig := make(map[string]interface{})
	fields["digitaloceanConfig"] = digitaloceanConfig

	digitaloceanConfig["AccessToken"] = access_token
	digitaloceanConfig["Region"] = "sfo1"
	digitaloceanConfig["Size"] = "1gb"
	digitaloceanConfig["Image"] = "ubuntu-14-04-x64"
	digitaloceanConfig["Ipv6"] = true
	digitaloceanConfig["Backups"] = false
	digitaloceanConfig["PrivateNetworking"] = true

	machine := &client.Machine{
		Data:             data,
		EngineInstallUrl: "https://test.docker.com/",
		Kind:             "machine",
		Driver:           "DigitalOcean",
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
