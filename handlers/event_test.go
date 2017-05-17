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

var (
	eventFlavorHostCreate = &events.Event{
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

	eventFlavorHostDelete1 = &events.Event{
		Name:         "physicalhost.remove;handler=goMachineService",
		ID:           "a439dbf2-7657-46ae-931a-e1b400f80fe0",
		ReplyTo:      "reply.4074420224106915443",
		ResourceID:   "1ph6",
		ResourceType: "physicalHost",

		Data: map[string]interface{}{},
	}

	eventFlavorHostBootstrap = &events.Event{
		Name:         "physicalhost.bootstrap;handler=goMachineService",
		ID:           "0b513573-57be-4905-84d7-4d4b6d8e9c37",
		ReplyTo:      "reply.3249066143597845078",
		ResourceID:   "1ph8",
		ResourceType: "physicalHost",

		Data: map[string]interface{}{
			"name":          "ivan-do3",
			"kind":          "machine",
			"externalId":    "921d6c24-d22b-4032-a1bd-b3a7318b402e",
			"rancherConfig": map[string]interface{}{"flavor": "digitalocean-sfo2-2gb"},
			"accountId":     5,
			"hostname":      "ivan-do3",
		},
	}
)

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

func TestDeleteMachine1(t *testing.T) {
	assert := require.New(t)

	err := PurgeMachine(eventFlavorHostDelete1, apiClient)
	assert.Nil(err)
}

func TestActivateMachine(t *testing.T) {
	assert := require.New(t)

	err := ActivateMachine(eventFlavorHostBootstrap, apiClient)
	assert.Nil(err)
}

func TestGetConnectionConfig(t *testing.T) {
	assert := require.New(t)

	machineDir := "/Users/ivan/.cattle/machine/machines/921d6c24-d22b-4032-a1bd-b3a7318b402e"
	machineName := "ivan-do3"

	conf, err := getConnectionConfig(machineDir, machineName)
	assert.Nil(err)
	assert.NotNil(conf)
}

func TestMachineExists(t *testing.T) {
	assert := require.New(t)

	exists, _ := machineExists("/Users/ivan/.cattle/machine/machines/fdccb274-92b2-4e77-ad79-f0a772622e80", "ivan-do1")
	assert.False(exists)
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
