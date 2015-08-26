package providers

import (
	"fmt"

	"github.com/rancher/go-rancher/client"
)

type Provider interface {
	HandleCreate(machine *client.Machine, machineDir string) error

	HandleError(msg string) string
}

type DefaultProvider struct {
}

func (*DefaultProvider) HandleCreate(machine *client.Machine, machineDir string) error {
	return nil
}

func (*DefaultProvider) HandleError(msg string) string {
	return msg
}

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
	defaultProvider := &DefaultProvider{}
	return defaultProvider
}
