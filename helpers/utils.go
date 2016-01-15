package helpers

func getWhitelistedDrivers(resourceData *ResourceData) []string {
	tempMap := make(map[string]string)
	for _, driver := range resourceData.Blacklist {
		tempMap[driver] = ""
	}
	whitelist := []string{}
	for _, driver := range resourceData.Drivers {
		if _, ok := tempMap[driver]; !ok {
			whitelist = append(whitelist, driver)
		}
	}
	return whitelist
}

func isBlacklisted(resourceData *ResourceData, driver string) bool {
	for _, disallowedDriver := range resourceData.Blacklist {
		if disallowedDriver == driver {
			return true
		}
	}
	return false
}
