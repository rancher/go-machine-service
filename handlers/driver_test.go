package handlers

import (
	"github.com/rancherio/go-machine-service/api"
	"github.com/rancherio/go-machine-service/events"
	tu "github.com/rancherio/go-machine-service/test_utils"
	"log"
	"strconv"
	"testing"
	"time"
)

func TestDriverSanity(t *testing.T) {
	log.Println("Handler sanity test passed")
}

func xTestDigitalOcean(t *testing.T) {
	accessToken := ""
	if accessToken == "" {
		log.Println("This is a manual test meant for locally exercising DigitalOcean integration. " +
			"Must set access token manually.")
		t.FailNow()
	}

	resourceId := "DO-" + strconv.FormatInt(time.Now().Unix(), 10)
	digOceanHost := &api.PhysicalHost{
		DigitaloceanConfig: map[string]interface{}{"accessToken": accessToken},
		Id:                 resourceId,
		ExternalId:         "ext-" + resourceId,
		Type:               "machineHost",
		Kind:               "machineHost",
		Driver:             "DigitalOcean",
	}

	event := &events.Event{
		ResourceId: resourceId,
		Id:         "event-id",
		ReplyTo:    "reply-to-id",
	}

	mockApiClient := &tu.MockApiClient{MockPhysicalHost: digOceanHost}

	replyCalled := false
	replyEventHandler := func(replyEvent *events.ReplyEvent) {
		replyCalled = true
	}

	CreateMachine(event, replyEventHandler, mockApiClient)
	if !replyCalled {
		tu.FailNowStackf(t, "Reply not called for event [%v]", event.Id)
	}
}
