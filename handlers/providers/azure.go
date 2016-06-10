package providers

import (
	b64 "encoding/base64"
	"os"
	"path/filepath"

	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/client"
)

func init() {
	azureHandler := &AzureHandler{}
	if err := RegisterProvider("azure", azureHandler); err != nil {
		log.Fatal("could not register azure provider")
	}
}

type AzureHandler struct {
}

func (*AzureHandler) HandleCreate(machine *client.Machine, machineDir string) error {
	var filename string
	fields := machine.Data["fields"]
	if fields == nil {
		return errors.New(machine.Driver + "Config does not exist on Machine " + machine.Id)
	}
	driverConfig := fields.(map[string]interface{})[machine.Driver+"Config"]
	if driverConfig == nil {
		return errors.New(machine.Driver + "Config does not exist on Machine " + machine.Id)
	}
	machineConfig := driverConfig.(map[string]interface{})
	if _, ok := machineConfig["subscriptionCert"]; ok {
		value := machineConfig["subscriptionCert"].(string)
		filename = "subscription-cert.pem"
		path, err := saveDataToFile(filename, value, machineDir)
		if err != nil {
			return err
		}
		machineConfig["subscriptionCert"] = path
	} else if _, ok := machineConfig["publishSettingsFile"]; ok {
		value := machineConfig["publishSettingsFile"].(string)
		filename = "publish-settings.xml"
		path, err := saveDataToFile(filename, value, machineDir)
		if err != nil {
			return err
		}
		machineConfig["publishSettingsFile"] = path
	}
	return nil
}

func (*AzureHandler) HandleError(msg string) string {
	return msg
}

func saveDataToFile(filename, data, machineDir string) (string, error) {
	f, err := os.Create(filepath.Join(machineDir, filename))
	defer f.Close()
	if err != nil {
		return "", err
	}

	var byteData []byte
	byteData, err = b64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", err
	}

	_, err = f.Write(byteData)
	if err != nil {
		return "", err
	}
	return f.Name(), nil
}
