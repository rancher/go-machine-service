package handlers

import (
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
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
	getMachine = func(id string, apiClient *client.RancherClient) (*client.MachineHost, error) {
		machine := &client.MachineHost{
			DigitaloceanConfig: client.DigitaloceanConfig{
				AccessToken: access_token,
			},
			ExternalId: "ext-" + id,
			Kind:       "machineHost",
			Driver:     "DigitalOcean",
		}
		machine.Id = id

		return machine, nil
	}

	getRegistrationUrl = func(accountId string, apiClient *client.RancherClient) (string, error) {
		return "http://1.2.3.4/v1", nil
	}

	publishReply = func(reply *client.Publish, apiClient *client.RancherClient) error { return nil }
}
