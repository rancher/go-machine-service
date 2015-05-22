package helpers

import (
	"encoding/json"
	"fmt"
)

func GenerateMachineJson(resourceData *ResourceData) error {
	resourceFieldStruct := make(ResourceFields)
	resourceFieldMap := make(ResourceFieldConfigs)
	resourceFieldStruct["resourceFields"] = resourceFieldMap
	for _, driver := range getWhitelistedDrivers(resourceData) {
		resourceFieldMap[driver+"Config"] = ResourceFieldConfig{Type: driver + "Config", Nullable: true}
	}
	jsonData, err := json.MarshalIndent(resourceFieldStruct, "", "    ")
	if nil != err {
		return fmt.Errorf("could not marshal machine data into json, err=%v", err)
	}
	return writeToFile(jsonData, "machine-generated.json")
}
