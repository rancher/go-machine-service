package handlers

import (
	"testing"

	"github.com/rancherio/go-rancher/client"
)

func TestBuildContainerConfig(t *testing.T) {
	machine := new(client.Machine)

	machine.ExternalId = "externalId"
	labels := make(map[string]interface{})

	labels["abc"] = "def"

	machine.Labels = labels
	config := buildContainerConfig([]string{}, machine, "rancher/agent", "0.7.8")

	for _, elem := range config.Env {
		if elem == "abc=def" {
			return
		}
	}
	t.Error("label is not being set!")
}
