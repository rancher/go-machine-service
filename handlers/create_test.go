package handlers

import (
	"strings"
	"testing"

	"github.com/rancher/go-rancher/client"
)

func TestBuildMachineNoEngineOptsCreateCommand(t *testing.T) {
	machine := new(client.Machine)
	machine.Driver = "rackspace"
	machine.RackspaceConfig = &client.RackspaceConfig{
		Username: "fakeUser",
		ApiKey:   "fakeAPiKey",
	}
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
	machine.EngineOpts = []string{"key1=val1", "key2=val2"}
	machine.EngineEnv = []string{"key3=val3"}
	machine.EngineInsecureRegistry = []string{}
	machine.EngineLabel = []string{"io.rancher.label=123"}
	machine.EngineRegistryMirror = []string{}
	machine.EngineStorageDriver = "deviceMapper"
	machine.RackspaceConfig = &client.RackspaceConfig{
		Username: "fakeUser",
		ApiKey:   "fakeAPiKey",
	}
	machine.Name = "fakeMachine"

	cmd, err := buildMachineCreateCmd(machine)
	if err != nil {
		t.Fatal("Error while building machine craete command", err)
	}

	if strings.Join(cmd, " ") != "create -d rackspace --engine-opt key1=val1 --engine-opt key2=val2 --engine-env key3=val3 --engine-label io.rancher.label=123 --engine-storage-driver deviceMapper --rackspace-api-key fakeAPiKey --rackspace-username fakeUser fakeMachine" {
		t.Error("Error building machine create command, got output", strings.Join(cmd, " "))
	}
}

func TestBuildMachineEngineOptsCommand1(t *testing.T) {
	engineOpts := []string{"key1=val1", "key2=val2"}

	cmd := buildEngineOpts("--engine-opt", engineOpts)

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
	engineOpts := []string{}

	cmd := buildEngineOpts("--engine-opt", engineOpts)

	engineOptCount := 0
	firstOptsFound := false
	secondOptsFound := false

	for _, elem := range cmd {
		if elem == "--engine-opt" {
			engineOptCount++
		} else if elem == "key1=val1" {
			firstOptsFound = true
		}
		if elem == "key2=val2" {
			secondOptsFound = true
		}
	}
	if engineOptCount != 0 || firstOptsFound || secondOptsFound {
		t.Error("Engine Opts is not being set!")
	}
}
