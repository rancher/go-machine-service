package handlers

import (
	"strings"
	"testing"

	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
)

func TestReplyForPhysicalHost(t *testing.T) {
	// Assert that when the event is for a phyiscal host and not a machine that
	// the create handler simply replies.
	event := &events.Event{
		ResourceID: "foo",
		ID:         "event-id",
		ReplyTo:    "reply-to-id",
	}
	mockAPIClient := &client.RancherClient{}
	mockAPIClient.Machine = &MockMachineOperations{}
	publishReply = func(reply *client.Publish, apiClient *client.RancherClient) error {
		if reply.Name != "reply-to-id" {
			t.Logf("%+v", reply)
			t.Fatalf("Reply not as expected: %+v", reply)
		}
		return nil
	}
	err := CreateMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}
}

type MockMachineOperations struct {
	client.MachineClient
}

func (c *MockMachineOperations) ById(id string) (*client.Machine, error) {
	// Return nil to indicate a 404
	return nil, nil
}

func TestBuildMachineNoEngineOptsCreateCommand(t *testing.T) {
	machine := new(client.Machine)
	machine.Driver = "rackspace"

	data := make(map[string]interface{})
	data["fields"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["rackspaceConfig"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["apiKey"] = "fakeAPiKey"
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["username"] = "fakeUser"

	machine.Data = data
	machine.Name = "fakeMachine"

	cmd, err := buildMachineCreateCmd(machine)
	if err != nil {
		t.Fatal("Error while building machine craete command", err)
	}

	if strings.Join(cmd, " ") != "create -d rackspace --rackspace-api-key fakeAPiKey --rackspace-username fakeUser fakeMachine" {
		t.Error("Error building machine create command, got output", strings.Join(cmd, " "))
	}
}

func TestBuildMachineCreateCommand(t *testing.T) {
	machine := new(client.Machine)
	machine.Driver = "rackspace"
	machine.EngineInstallUrl = "test.com"
	machine.EngineOpt = map[string]interface{}{"key1": "val1", "key2": "val2"}
	machine.EngineEnv = map[string]interface{}{"key3": "val3"}
	machine.EngineInsecureRegistry = []string{}
	machine.EngineLabel = map[string]interface{}{"io.rancher.label": "123"}
	machine.EngineRegistryMirror = []string{}
	machine.EngineStorageDriver = "deviceMapper"

	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	rackspaceConfig := make(map[string]interface{})
	fields["rackspaceConfig"] = rackspaceConfig
	rackspaceConfig["apiKey"] = "fakeAPiKey"
	rackspaceConfig["username"] = "fakeUser"

	machine.Data = data
	machine.Name = "fakeMachine"

	cmd, err := buildMachineCreateCmd(machine)
	if err != nil {
		t.Fatal("Error while building machine craete command", err)
	}

	command := strings.Join(cmd, " ")
	// We have two engine opts in a map and maps are randomized, so have to look for both orders of engine-opt
	if command != "create -d rackspace --engine-install-url test.com --engine-opt key1=val1 --engine-opt key2=val2 --engine-env key3=val3 --engine-label io.rancher.label=123 --engine-storage-driver deviceMapper --rackspace-api-key fakeAPiKey --rackspace-username fakeUser fakeMachine" && command != "create -d rackspace --engine-install-url test.com --engine-opt key2=val2 --engine-opt key1=val1 --engine-env key3=val3 --engine-label io.rancher.label=123 --engine-storage-driver deviceMapper --rackspace-api-key fakeAPiKey --rackspace-username fakeUser fakeMachine" {
		t.Error("Error building machine create command, got output", strings.Join(cmd, " "))
	}
}

func TestBuildMacineCreateCommandWithInterfaceLists(t *testing.T) {
	machine := new(client.Machine)
	machine.Driver = "rackspace"
	machine.EngineInstallUrl = "test.com"
	machine.EngineEnv = map[string]interface{}{"key3": "val3"}
	machine.EngineInsecureRegistry = []string{}
	machine.EngineLabel = map[string]interface{}{"io.rancher.label": "123"}
	machine.EngineRegistryMirror = []string{}
	machine.EngineStorageDriver = "deviceMapper"

	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	rackspaceConfig := make(map[string]interface{})
	fields["rackspaceConfig"] = rackspaceConfig
	rackspaceConfig["apiKey"] = "fakeAPiKey"
	rackspaceConfig["username"] = "fakeUser"
	rackspaceConfig["interfaceList"] = []interface{}{"str1", "str2"}

	machine.Data = data
	machine.Name = "fakeMachine"

	cmd, err := buildMachineCreateCmd(machine)
	if err != nil {
		t.Fatal("Error while building machine craete command", err)
	}

	command := strings.Join(cmd, " ")
	// We have two engine opts in a map and maps are randomized, so have to look for both orders of engine-opt
	if command != "create -d rackspace --engine-install-url test.com --engine-env key3=val3 --engine-label io.rancher.label=123 --engine-storage-driver deviceMapper --rackspace-api-key fakeAPiKey --rackspace-interface-list str1 --rackspace-interface-list str2 --rackspace-username fakeUser fakeMachine" {
		t.Error("Error building machine create command, got output", strings.Join(cmd, " "))
	}

}

func TestBuildMachineEngineOptsCommand1(t *testing.T) {
	engineOpts := map[string]interface{}{"key1": "val1", "key2": "val2"}

	cmd := buildEngineOpts("--engine-opt", mapToSlice(engineOpts))

	engineOptCount := 0
	firstOptsFound := false
	secondOptsFound := false

	for _, elem := range cmd {
		if elem == "--engine-opt" {
			engineOptCount++
		}
		if elem == "key1=val1" {
			firstOptsFound = true
		}
		if elem == "key2=val2" {
			secondOptsFound = true
		}
	}
	if engineOptCount != 2 || !firstOptsFound || !secondOptsFound {
		t.Error("Engine Opts is not being set!")
	}
}

func TestBuildMachineEngineOptsCommand2(t *testing.T) {
	storageDriver := []string{"devicemapper"}

	cmd := buildEngineOpts("--engine-storage-driver", storageDriver)

	engineOptCount := 0
	firstOptsFound := false
	secondOptsFound := false

	for _, elem := range cmd {
		if elem == "--engine-storage-driver" {
			engineOptCount++
		} else if elem == "devicemapper" {
			firstOptsFound = true
		} else {
			secondOptsFound = true
		}
	}

	if engineOptCount != 1 || !firstOptsFound || secondOptsFound {
		t.Error("Engine Opts is not being set properly!")
	}
}

func TestBuildMachineEngineOptsCommand3(t *testing.T) {
	engineOpts := map[string]interface{}{}

	cmd := buildEngineOpts("--engine-opt", mapToSlice(engineOpts))
	if len(cmd) != 0 {
		t.Error("Expected empty command")
	}
}
