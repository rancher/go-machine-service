package handlers

import (
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestAzure(t *testing.T) {
	subscriptionID := os.Getenv("SUBSCRIPTION_ID")
	if subscriptionID == "" {
		t.Log("Skipping Azure test.")
		return
	}

	subscriptionCert := os.Getenv("SUBSCRIPTION_CERT")
	if subscriptionCert == "" {
		t.Log("Skipping Azure test.")
		return
	}

	setupAZ(subscriptionID, subscriptionCert)

	resourceID := "AZ-" + strconv.FormatInt(time.Now().Unix(), 10)
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

func setupAZ(subscriptionID, subscriptionCert string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	azureConfig := make(map[string]interface{})
	fields["azureConfig"] = azureConfig
	azureConfig["SubscriptionCert"] = subscriptionCert
	azureConfig["SubscriptionId"] = subscriptionID

	machine := &client.Machine{
		Data:   data,
		Kind:   "machine",
		Driver: "azure",
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
