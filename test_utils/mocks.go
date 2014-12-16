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
			Id:         id,
			ExternalId: "ext-" + id,
			Type:       "virtualBoxHost",
			Kind:       "virtualBoxHost",
		}, nil
	}

	return m.MockPhysicalHost, nil
}
