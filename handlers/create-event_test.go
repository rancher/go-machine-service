package handlers

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/event-subscriber/events"
	_ "github.com/rancher/go-machine-service/logging"
	"github.com/rancher/go-rancher/v2"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

var eventFlavorHostCreate = &events.Event{
	Name:         "physicalhost.create;handler=goMachineService",
	ID:           "873c98c6-50c3-4e5e-b3ca-aa37f4020588",
	ReplyTo:      "reply.4427210190781767296",
	ResourceID:   "1ph7",
	ResourceType: "physicalHost",

	Data: map[string]interface{}{
		"name":          "ivan-do1",
		"kind":          "machine",
		"externalId":    "8bb270e3-5920-412a-a4c4-3951e16ccbe9",
		"rancherConfig": map[string]interface{}{"flavor": "digitalocean-sfo2-2gb"},
		"accountId":     5,
		"hostname":      "ivan-do1",
	},
}

var apiClient, err = client.NewRancherClient(&client.ClientOpts{
	Timeout:   time.Second * 30,
	Url:       "http://localhost:8080/v2-beta",
	AccessKey: "service",
	SecretKey: "servicepass",
})

func TestCreateMachine(t *testing.T) {
	assert := require.New(t)

	err := CreateMachine(eventFlavorHostCreate, apiClient)
	assert.Nil(err)
}

var fields = map[string]interface{}{
	"digitaloceanConfig": map[string]interface{}{
		"accessToken": "7afcc8d639e1087c9f8d283d7f35e72902d32ceca6237f0c48fca10d4385627d",
	},
}

func TestAddProviderFields(t *testing.T) {
	assert := require.New(t)

	rancherConfig := map[string]interface{}{}

	rancherConfigAppendFieldsData(rancherConfig, fields, "digitalocean")
	logrus.Warnf("%+v", rancherConfig)
	assert.Equal(fields["digitaloceanConfig"].(map[string]interface{})["accessToken"], rancherConfig["digitaloceanAccessToken"])
}
