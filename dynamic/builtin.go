package dynamic

import (
	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	"github.com/rancher/go-rancher/v2"
)

var (
	ignoredDrivers = map[string]bool{
		"none":         true,
		"virtualbox":   true,
		"vmwarefusion": true,
		"hyperv":       true,
	}
)

func SyncBuiltin() error {
	apiClient, err := getClient()
	if err != nil {
		return err
	}

	opts := client.NewListOpts()
	opts.Filters["state_ne"] = "removed"

	b, err := apiClient.MachineDriver.List(opts)
	if err != nil {
		return err
	}

	installed := map[string]client.MachineDriver{}

	for _, driver := range b.Data {
		if driver.Builtin {
			installed[driver.Name] = driver
		}
		if driver.State == "inactive" && driver.DefaultActive {
			logger.Infof("Activating driver %s", driver.Name)
			apiClient.MachineDriver.ActionActivate(&driver)
		}
	}

	for _, driver := range localbinary.CoreDrivers {
		if _, ok := installed[driver]; !ok && !ignoredDrivers[driver] {
			logger.Infof("Installing builtin driver %s", driver)
			apiClient.MachineDriver.Create(&client.MachineDriver{
				Name:    driver,
				Builtin: true,
				Url:     "local://",
			})
		}
		delete(installed, driver)
	}

	for _, driver := range installed {
		logger.Infof("Deleting old builtin driver %s", driver)
		apiClient.MachineDriver.Delete(&driver)
	}

	return nil
}
