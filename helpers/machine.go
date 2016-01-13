package helpers

import (
	"encoding/json"
)

func GenerateMachineJson(drivers []string) (string, error) {
	resourceFieldStruct := make(ResourceFields)
	resourceFieldMap := make(ResourceFieldConfigs)
	resourceFieldStruct["resourceFields"] = resourceFieldMap
	for _, driver := range drivers {
		resourceFieldMap[driver + "Config"] = ResourceFieldConfig{
			Type: driver + "Config",
			Nullable: true,
			Create: true,
		}
	}
	resourceFieldMap["authCertificateAuthority"] = ResourceFieldConfig{
		Type: "string",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["authKey"] = ResourceFieldConfig{
		Type: "string",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["extractedConfig"] = ResourceFieldConfig{
		Type: "string",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["labels"] = ResourceFieldConfig{
		Type: "map[string]",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["engineInstallUrl"] = ResourceFieldConfig{
		Type: "string",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["dockerVersion"] = ResourceFieldConfig{
		Type: "string",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["engineOpt"] = ResourceFieldConfig{
		Type: "map[string]",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["engineInsecureRegistry"] = ResourceFieldConfig{
		Type: "array[string]",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["engineRegistryMirror"] = ResourceFieldConfig{
		Type: "array[string]",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["engineLabel"] = ResourceFieldConfig{
		Type: "map[string]",
		Nullable: true,
		Create: true,
	}
	resourceFieldMap["engineStorageDriver"] = ResourceFieldConfig{
		Type: "string",
		Nullable: true,
		Update: true,
		Create: true,
	}
	resourceFieldMap["engineEnv"] = ResourceFieldConfig{
		Type: "map[string]",
		Nullable: true,
		Create: true,
		Update: true,
	}

	jsonData, err := json.MarshalIndent(resourceFieldStruct, "", "    ")
	return string(jsonData), err
}

func uploadMachineSchema(drivers []string) error {
	jsonData, err := GenerateMachineJson(drivers)
	if (err != nil) {
		return err
	}
	return uploadDynamicSchema("machine", jsonData, "physicalHost", []string{"service", "member",
		"owner", "project", "admin"})
}
