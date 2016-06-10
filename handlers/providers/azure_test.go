package providers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/go-rancher/client"
)

/* This file contains unit tests for azure.go
 */

func TestAzureHandler(t *testing.T) {
	machine := new(client.Machine)
	machineDir := "."

	machine.Driver = "azure"
	data := make(map[string]interface{})
	machine.Data = data
	fields := make(map[string]interface{})
	data["fields"] = fields
	azureConfig := make(map[string]interface{})
	fields["azureConfig"] = azureConfig
	azureConfig["subscriptionCert"] = "QXp1cmVDb25maWc=" //base64 encoding of 'AzureConfig'

	azureHandler := &AzureHandler{}

	err := azureHandler.HandleCreate(machine, machineDir)

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

	if azureConfig["subscriptionCert"] != filename {
		t.Fatalf("parameter is wrong, got %s", azureConfig["subscriptionCert"])
	}

}

func TestAzureHandler_DM_0_7(t *testing.T) {
	machine := new(client.Machine)
	machineDir := "."

	machine.Driver = "azure"
	data := make(map[string]interface{})
	machine.Data = data
	fields := make(map[string]interface{})
	data["fields"] = fields
	azureConfig := make(map[string]interface{})
	fields["azureConfig"] = azureConfig

	azureHandler := &AzureHandler{}

	err := azureHandler.HandleCreate(machine, machineDir)

	if err != nil {
		t.Errorf("not Docker Machine 0.7.0 ready, err=%v", err)
		return
	}
}
