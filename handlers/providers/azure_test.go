package providers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancherio/go-rancher/client"
)

/* This file contains unit tests for azure.go
 */

func TestAzureHandler(t *testing.T) {
	machine := new(client.Machine)
	machineDir := "."

	machine.AzureConfig = new(client.AzureConfig)
	machine.AzureConfig.SubscriptionCert = "QXp1cmVDb25maWc=" //base64 encoding of 'AzureConfig'

	err := azureHandler(machine, machineDir)

	if err != nil {
		t.Errorf("could not save subscriptionCert to path, err=%v", err)
		return
	}

	var f *os.File
	filename := filepath.Join(machineDir, "subscription-cert.pem")
	f, err = os.Open(filename)
	defer os.Remove(filename)

	if err != nil {
		t.Errorf("could not open saved file, err=%v", err)
		return
	}

	b := make([]byte, 11)
	f.Read(b)
	if string(b) != "AzureConfig" {
		t.Errorf("Saved data [AzureConfig] is not the same as actual data [%s]", string(b))
	}

}
