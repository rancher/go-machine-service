package dynamicDrivers

import (
	"github.com/rancher/go-rancher/client"
	"strings"
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
