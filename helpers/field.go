package helpers

import (
	"encoding/json"
	"fmt"
)

func GenerateFieldJsons(resourceData *ResourceData) error {
	for _, driver := range getWhitelistedDrivers(resourceData) {
		fieldJsonData, ok := resourceData.ResourceMap[driver]
		if !ok {
			return fmt.Errorf("could not find resourceData for driver=%s", driver)
		}
		fieldJson, err := json.MarshalIndent(fieldJsonData, "", "    ")
		if err != nil {
			return fmt.Errorf("could not marshall data into json, driver=%s err=%v", driver, err)
		}
		err = writeToFile(fieldJson, driver+"Config.json")
		if err != nil {
			return fmt.Errorf("error writing field json for driver=%s err=%v", driver, err)
		}
	}
	return nil
}
