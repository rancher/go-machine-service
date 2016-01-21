package helpers

import (
	"encoding/json"
	"fmt"
)

func GenerateDocJsons(resourceData *ResourceData) error {
	for _, driver := range getWhitelistedDrivers(resourceData) {
		docJsonData, ok := resourceData.DocumentationMap[driver]
		if !ok {
			return fmt.Errorf("could not find documentationData for driver=%s", driver)
		}
		docJson, err := json.MarshalIndent(docJsonData, "", "    ")
		if err != nil {
			return fmt.Errorf("could not marshall data into json, driver=%s err=%v", driver, err)
		}
		err = writeToFile(docJson, driver+"Config-doc.json")
		if err != nil {
			return fmt.Errorf("error writing field json for driver=%s err=%v", driver, err)
		}
	}
	return nil
}
