package providers

import (
	b64 "encoding/base64"
	"os"
	"path/filepath"

	"errors"

	"github.com/rancher/go-machine-service/logging"
	"github.com/rancher/go-rancher/v3"
)

var logger = logging.Logger()

func init() {
	azureHandler := &AzureHandler{}
	if err := RegisterProvider("azure", azureHandler); err != nil {
		logger.Fatal("could not register azure provider")
	}
}

type AzureHandler struct {
}

func (*AzureHandler) HandleCreate(host *client.Host, hostDir string) error {
	var filename string
	fields := host.Data["fields"]
	if fields == nil {
		return errors.New("AzureConfig does not exist on Machine " + host.Id)
	}
	driverConfig := fields.(map[string]interface{})[host.Driver+"Config"]
	if driverConfig == nil {
		return errors.New("AzureConfig does not exist on Machine " + host.Id)
	}
	machineConfig := driverConfig.(map[string]interface{})
	if _, ok := machineConfig["subscriptionCert"]; ok {
		value := machineConfig["subscriptionCert"].(string)
		filename = "subscription-cert.pem"
		path, err := saveDataToFile(filename, value, hostDir)
		if err != nil {
			return err
		}
		machineConfig["subscriptionCert"] = path
	} else if _, ok := machineConfig["publishSettingsFile"]; ok {
		value := machineConfig["publishSettingsFile"].(string)
		filename = "publish-settings.xml"
		path, err := saveDataToFile(filename, value, hostDir)
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
