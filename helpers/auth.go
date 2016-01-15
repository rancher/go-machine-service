package helpers

import (
	"encoding/json"
	"fmt"

	"errors"
)

func GenerateAuthJsons(resourceData *ResourceData) error {
	err := generateAuthJson(resourceData, "project", "cr")
	if err != nil {
		return err
	}
	return generateAuthJson(resourceData, "user", "r")
}

func generateAuthJson(resourceData *ResourceData, prefix, perm string) error {
	authorizeMap := make(map[string]interface{})
	fieldData := make(map[string]string)
	authorizeMap["authorize"] = fieldData
	for _, driver := range getWhitelistedDrivers(resourceData) {
		fieldData["machine."+driver+"Config"] = perm
		fieldData[driver+"Config"] = perm
	}
	for driver, resourceDataUnit := range resourceData.ResourceMap {
		if isBlacklisted(resourceData, driver) {
			continue
		}
		resourceMap, ok := resourceDataUnit["resourceFields"]
		if !ok {
			return errors.New("Could not find resource data for driver=" + driver)
		}
		for key := range resourceMap {
			fieldData[driver+"Config."+key] = perm
		}
	}

	jsonData, err := json.MarshalIndent(authorizeMap, "", "    ")
	if err != nil {
		return fmt.Errorf("error encoding the authorize data into json, err=%v", err)
	}
	return writeToFile(jsonData, prefix+"-auth.json")
}
