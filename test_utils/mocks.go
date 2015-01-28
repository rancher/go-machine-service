package test_utils

import (
	"github.com/rancherio/go-machine-service/api"
)

type MockApiClient struct {
	MockPhysicalHost *api.PhysicalHost
}

func (m *MockApiClient) GetPhysicalHost(id string) (*api.PhysicalHost, error) {
	if m.MockPhysicalHost == nil {
		return &api.PhysicalHost{
			Id:               id,
			ExternalId:       "ext-" + id,
			Type:             "machineHost",
			Kind:             "machineHost",
			Driver:           "VirtualBox",
			VirtualboxConfig: map[string]interface{}{},
		}, nil
	}

	return m.MockPhysicalHost, nil
}
