package handlers

import (
	"strings"
	"testing"

	"github.com/rancher/go-rancher/v3"
)

type MockMachineOperations struct {
	client.HostClient
}

func (c *MockMachineOperations) ById(id string) (*client.Host, error) {
	// Return nil to indicate a 404
	return nil, nil
}

func TestBuildMachineNoEngineOptsCreateCommand(t *testing.T) {
	machine := new(client.Host)
	machine.Driver = "rackspace"

	data := make(map[string]interface{})
	data["fields"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["rackspaceConfig"] = make(map[string]interface{})
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["apiKey"] = "fakeAPiKey"
	data["fields"].(map[string]interface{})["rackspaceConfig"].(map[string]interface{})["username"] = "fakeUser"

	machine.Data = data
	machine.Name = "fakeMachine"

	cmd, err := buildMachineCreateCmd(machine, machine.Driver)
	if err != nil {
		t.Fatal("Error while building machine craete command", err)
	}

	if strings.Join(cmd, " ") != "create -d rackspace --rackspace-api-key fakeAPiKey --rackspace-username fakeUser fakeMachine" {
		t.Error("Error building machine create command, got output", strings.Join(cmd, " "))
	}
}

func TestBuildMachineCreateCommand(t *testing.T) {
	host := new(client.Host)
	host.Driver = "rackspace"
	host.EngineInstallUrl = "test.com"
	host.EngineOpt = map[string]interface{}{"key1": "val1", "key2": "val2"}
	host.EngineEnv = map[string]interface{}{"key3": "val3"}
	host.EngineInsecureRegistry = []string{}
	host.EngineLabel = map[string]interface{}{"io.rancher.label": "123"}
	host.EngineRegistryMirror = []string{}
	host.EngineStorageDriver = "deviceMapper"

	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	rackspaceConfig := make(map[string]interface{})
	fields["rackspaceConfig"] = rackspaceConfig
	rackspaceConfig["apiKey"] = "fakeAPiKey"
	rackspaceConfig["username"] = "fakeUser"

	host.Data = data
	host.Name = "fakeMachine"

	cmd, err := buildMachineCreateCmd(host, host.Driver)
	if err != nil {
		t.Fatal("Error while building host craete command", err)
	}

	command := strings.Join(cmd, " ")
	// We have two engine opts in a map and maps are randomized, so have to look for both orders of engine-opt
	if command != "create -d rackspace --engine-install-url test.com --engine-opt key1=val1 --engine-opt key2=val2 --engine-env key3=val3 --engine-label io.rancher.label=123 --engine-storage-driver deviceMapper --rackspace-api-key fakeAPiKey --rackspace-username fakeUser fakeMachine" && command != "create -d rackspace --engine-install-url test.com --engine-opt key2=val2 --engine-opt key1=val1 --engine-env key3=val3 --engine-label io.rancher.label=123 --engine-storage-driver deviceMapper --rackspace-api-key fakeAPiKey --rackspace-username fakeUser fakeMachine" {
		t.Error("Error building machine create command, got output", strings.Join(cmd, " "))
	}
}

func TestBuildMacineCreateCommandWithInterfaceLists(t *testing.T) {
	host := new(client.Host)
	host.Driver = "rackspace"
	host.EngineInstallUrl = "test.com"
	host.EngineEnv = map[string]interface{}{"key3": "val3"}
	host.EngineInsecureRegistry = []string{}
	host.EngineLabel = map[string]interface{}{"io.rancher.label": "123"}
	host.EngineRegistryMirror = []string{}
	host.EngineStorageDriver = "deviceMapper"

	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	rackspaceConfig := make(map[string]interface{})
	fields["rackspaceConfig"] = rackspaceConfig
	rackspaceConfig["apiKey"] = "fakeAPiKey"
	rackspaceConfig["username"] = "fakeUser"
	rackspaceConfig["interfaceList"] = []interface{}{"str1", "str2"}

	host.Data = data
	host.Name = "fakeMachine"

	cmd, err := buildMachineCreateCmd(host, host.Driver)
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

func TestBuildContainerConfig(t *testing.T) {
	machine := new(client.Host)

	machine.ExternalId = "externalId"
	labels := make(map[string]interface{})

	labels["abc"] = "def"
	labels["foo"] = "bar"

	machine.Labels = labels
	config := buildContainerConfig([]string{}, machine, "rancher/agent", "0.7.8", "")

	for _, elem := range config.Env {
		if elem == "CATTLE_HOST_LABELS=abc=def&foo=bar" || elem == "CATTLE_HOST_LABELS=foo=bar&abc=def" {
			return
		}
	}
	t.Error("label is not being set!")
}
