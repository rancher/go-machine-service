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
	accessToken := os.Getenv("DIGITALOCEAN_KEY")
	if accessToken == "" {
		t.Log("Skipping Digital Ocean test.")
		return
	}
	setupDO(accessToken)

	resourceID := "DO-" + strconv.FormatInt(time.Now().Unix(), 10)
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

func setupDO(accessToken string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	digitaloceanConfig := make(map[string]interface{})
	fields["digitaloceanConfig"] = digitaloceanConfig

	digitaloceanConfig["accessToken"] = accessToken
	digitaloceanConfig["region"] = "sfo1"
	digitaloceanConfig["size"] = "1gb"
	digitaloceanConfig["image"] = "ubuntu-14-04-x64"
	digitaloceanConfig["ipv6"] = true
	digitaloceanConfig["backups"] = false
	digitaloceanConfig["privateNetworking"] = true

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

	getRegistrationURLAndImage = func(accountId string, apiClient *client.RancherClient) (string, string, string, error) {
		return "http://1.2.3.4/v1", "rancher/agent", "v0.7.6", nil
	}

	publishReply = buildMockPublishReply(machine)
	publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {}
}
