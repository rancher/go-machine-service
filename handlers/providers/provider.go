package providers

import (
	"fmt"

	"github.com/rancherio/go-rancher/client"
)

type Provider func(machine *client.Machine, machineDir string) error

var (
	providers map[string]Provider
)

func RegisterProvider(name string, provider Provider) error {
	if providers == nil {
		providers = make(map[string]Provider)
	}
	if _, exists := providers[name]; exists {
		return fmt.Errorf("provider already registered")
	}
	providers[name] = provider
	return nil
}

func GetProviderHandler(name string) Provider {
	if provider, ok := providers[name]; ok {
		return provider
	}
	return nil
}
