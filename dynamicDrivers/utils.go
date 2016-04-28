package dynamicDrivers

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/client"
	"strings"
	"time"
)

func isBlacklisted(blackList []string, driver string) bool {
	for _, blackListedDriver := range blackList {
		if blackListedDriver == driver {
			return true
		}
	}
	return false
}

func getBlackListSetting(apiClient *client.RancherClient) (*client.Setting, error) {
	return apiClient.Setting.ById("machine.driver.blacklist")
}

func getBlackListedDrivers(apiClient *client.RancherClient) ([]string, error) {
	setting, err := getBlackListSetting(apiClient)
	if err != nil {
		return nil, err
	}
	return strings.Split(setting.Value, ";"), nil
}

func waitSuccessDriver(driver client.MachineDriver, apiClient *client.RancherClient) error {

	timeout := time.After(30 * time.Second)
	tick := time.Tick(time.Millisecond * 500)
	gotDriver, err := apiClient.MachineDriver.ById(driver.Id)
	if err != nil {
		return err
	}
	for gotDriver.Transitioning == "yes" {
		select {
		case <-timeout:
			return fmt.Errorf("Timed out waiting for MachineDriver %v to be done.", driver.Id)
		case <-tick:
			gotDriver, err = apiClient.MachineDriver.ById(driver.Id)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func waitSuccessSchema(schema client.DynamicSchema, apiClient *client.RancherClient) error {

	timeout := time.After(5 * time.Minute)
	tick := time.Tick(time.Millisecond * 500)
	gotSchema, err := apiClient.DynamicSchema.ById(schema.Id)
	if err != nil {
		return err
	}
	for gotSchema.Transitioning == "yes" {
		select {
		case <-timeout:
			log.Debugf("Timedout waiting for Schema to activate. %v", schema.Name)
			return fmt.Errorf("Timed out waiting for Schema %v to be done.", schema.Id)
		case <-tick:
			gotSchema, err = apiClient.DynamicSchema.ById(schema.Id)
			if err != nil {
				log.Debugf("While waiting for schema : %v to succeed got and error.", schema.Name)
				return err
			}
		}
	}
	log.Debugf("Schema %v successfully uploaded.", schema.Name)
	return nil
}
