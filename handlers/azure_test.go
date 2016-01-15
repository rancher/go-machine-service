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
	subscriptionId := os.Getenv("SUBSCRIPTION_ID")
	if subscriptionId == "" {
		t.Log("Skipping Azure test.")
		return
	}

	subscriptionCert := os.Getenv("SUBSCRIPTION_CERT")
	if subscriptionCert == "" {
		t.Log("Skipping Azure test.")
		return
	}

	setupAZ(subscriptionId, subscriptionCert)

	resourceId := "AZ-" + strconv.FormatInt(time.Now().Unix(), 10)
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

func setupAZ(subscription_id, subscription_cert string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	data["fields"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["azureConfig"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["azureConfig"].(map[string]interface{})["SubscriptionId"] = subscription_id
	data["fields"].(map[string]interface{})["azureConfig"].(map[string]interface{})["SubscriptionCert"] = subscription_cert

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

	getRegistrationUrlAndImage = func(accountId string, apiClient *client.RancherClient) (string, string, string, error) {
		return "http://1.2.3.4/v1", "rancher/agent", "v0.7.6", nil
	}

	publishReply = buildMockPublishReply(machine)
	publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {}
}
