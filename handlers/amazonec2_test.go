package handlers

import (
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestAmazonec2(t *testing.T) {
	accessKey := os.Getenv("EC2_ACCESS_KEY")
	secretKey := os.Getenv("EC2_SECRET_KEY")
	vpcId := os.Getenv("EC2_VPC_ID")
	subnetId := os.Getenv("EC2_SUBNET_ID")

	if accessKey == "" || secretKey == "" || vpcId == "" || subnetId == "" {
		t.Log("Skipping Amazon EC2 Test.")
		return
	}
	setup(accessKey, secretKey, vpcId, subnetId)

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

func setup(accessKey string, secretKey string, vpcId string, subnetId string) {
	// TODO Replace functions during teardown.
	machine := &client.Machine{
		Amazonec2Config: client.Amazonec2Config{
			AccessKey: accessKey,
			SecretKey: secretKey,
			VpcId:     vpcId,
			SubnetId:  subnetId,
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

	getRegistrationUrl = func(accountId string, apiClient *client.RancherClient) (string, error) {
		return "http://1.2.3.4/v1", nil
	}

	publishReply = buildMockPublishReply(machine)
	publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {}
}
