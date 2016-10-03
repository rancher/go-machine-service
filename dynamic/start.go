package dynamic

import (
	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/v2"
)

func DownloadAllDrivers() error {
	logrus.Info("Installing builtin drivers")
	if err := SyncBuiltin(); err != nil {
		return err
	}

	logrus.Info("Downloading all drivers")
	apiClient, err := getClient()
	if err != nil {
		return err
	}

	opts := client.NewListOpts()
	opts.Filters["state"] = "active"

	drivers, err := apiClient.MachineDriver.List(opts)
	if err != nil {
		return err
	}

	for _, driverInfo := range drivers.Data {
		driver := NewDriver(driverInfo.Builtin, driverInfo.Name, driverInfo.Url, driverInfo.Checksum)
		err := driver.Stage()
		if err == nil {
			err = driver.Install()
		}

		if err != nil {
			logrus.Errorf("Failed to download/install driver %s: %v", driverInfo.Name, err)
			if _, err := apiClient.MachineDriver.ActionReactivate(&driverInfo); err != nil {
				return err
			}
		}
	}

	logrus.Info("Done downloading all drivers")
	return nil
}
