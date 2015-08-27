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
	vpcId := os.Getenv("EC2_VPC_ID")
	zone := os.Getenv("EC2_ZONE")

	if accessKey == "" || secretKey == "" || vpcId == "" {
		t.Log("Skipping Amazon EC2 Test.")
		return
	}
	setup(accessKey, secretKey, vpcId, zone)

	resourceId := "EC2-" + strconv.FormatInt(time.Now().Unix(), 10)
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

func setup(accessKey string, secretKey string, vpcId string, zone string) {
	// TODO Replace functions during teardown.
	machine := &client.Machine{
		Amazonec2Config: &client.Amazonec2Config{
			AccessKey: accessKey,
			SecretKey: secretKey,
			VpcId:     vpcId,
			Zone:      zone,
		},
		Kind:   "machine",
		Driver: "Amazonec2",
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
