package dynamic

import (
	"time"

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

	// this code block waits for handler to activate on the cattle side
	// this is necessary to make sure machine drivers show up when adding hosts
Loop:
	for {
		opts := client.NewListOpts()
		collection, _ := apiClient.ExternalHandlerProcess.List(opts)
		for _, event := range collection.Data {
			time.Sleep(3 * time.Second)
			if event.Name == "machinedriver.activate" && event.State == "active" {
				logger.Infoln("machinedriver.activate event detected")
				break Loop
			}
			logger.Infoln("Waiting for machinedriver.activate event")
		}
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
