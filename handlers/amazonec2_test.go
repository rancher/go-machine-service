package handlers

import (
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestAmazonec2(t *testing.T) {
	accessKey := os.Getenv("EC2_ACCESS_KEY")
	secretKey := os.Getenv("EC2_SECRET_KEY")
	vpcID := os.Getenv("EC2_VPC_ID")
	zone := os.Getenv("EC2_ZONE")

	if accessKey == "" || secretKey == "" || vpcID == "" {
		t.Log("Skipping Amazon EC2 Test.")
		return
	}
	setup(accessKey, secretKey, vpcID, zone)

	resourceID := "EC2-" + strconv.FormatInt(time.Now().Unix(), 10)
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

func setup(accessKey string, secretKey string, vpcID string, zone string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	amazonec2Config := make(map[string]interface{})
	fields["amazonec2Config"] = amazonec2Config
	amazonec2Config["accessKey"] = accessKey
	amazonec2Config["secretKey"] = secretKey
	amazonec2Config["vpcId"] = vpcID
	amazonec2Config["zone"] = zone

	machine := &client.Machine{
		Data:   data,
		Kind:   "machine",
		Driver: "Amazonec2",
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
