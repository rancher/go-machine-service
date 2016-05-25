package handlers

import (
	"testing"

	"github.com/rancher/go-rancher/client"
)

func TestBuildContainerConfig(t *testing.T) {
	machine := new(client.Machine)

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
