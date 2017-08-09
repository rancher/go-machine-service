package dynamic

import (
	"github.com/rancher/go-rancher/v3"
)

func DriverSchemaVersion(apiClient *client.RancherClient) (string, error) {
	url := ""
	setting, err := apiClient.Setting.ById("service.package.docker.machine.url")
	if err != nil {
		return "", err
	}
	if setting != nil {
		url = setting.Value
	}
	return url, nil
}

func ReactivateOldDrivers() error {
	apiClient, err := getClient()
	if err != nil {
		return err
	}

	version, err := DriverSchemaVersion(apiClient)
	if err != nil {
		return err
	}

	opts := client.NewListOpts()
	opts.Filters["state"] = "active"

	drivers, err := apiClient.MachineDriver.List(opts)
	if err != nil {
		return err
	}

	for _, driver := range drivers.Data {
		if driver.SchemaVersion != version {
			logger.Infof("Updating driver %s from %s => %s", driver.Name, driver.SchemaVersion, version)
			_, err := apiClient.MachineDriver.ActionReactivate(&driver)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func DownloadAllDrivers() error {
	logger.Info("Installing builtin drivers")
	if err := SyncBuiltin(); err != nil {
		return err
	}

	logger.Info("Downloading all drivers")
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
			logger.Errorf("Failed to download/install driver %s: %v", driverInfo.Name, err)
			if _, err := apiClient.MachineDriver.ActionReactivate(&driverInfo); err != nil {
				return err
			}
		}
	}

	logger.Info("Done downloading all drivers")
	return nil
}
