package providers

import (
	b64 "encoding/base64"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-rancher/client"
)

func init() {
	if err := RegisterProvider("azure", azureHandler); err != nil {
		log.Fatal("could not register azure provider")
	}
}

func azureHandler(machine *client.Machine, machineDir string) error {
	var data *string
	var filename string
	if machine.AzureConfig.SubscriptionCert != "" {
		data = &machine.AzureConfig.SubscriptionCert
		filename = "subscription-cert.pem"
	} else {
		data = &machine.AzureConfig.PublishSettingsFile
		filename = "publish-settings.xml"
	}
	path, err := saveDataToFile(filename, *data, machineDir)
	if err != nil {
		return err
	}
	*data = path
	return nil
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
