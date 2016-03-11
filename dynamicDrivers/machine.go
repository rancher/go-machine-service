package dynamicDrivers

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"strings"
)

func uploadMachineSchema(drivers []string) error {
	log.Debug("Uploading machine jsons.")
	err := uploadMachineServiceJSON(drivers)
	err2 := uploadMachineProjectJSON(drivers)
	err3 := uploadMachineUserJSON(drivers)
	err4 := uploadMachineReadOnlyJSON(drivers)
	if err != nil || err2 != nil || err3 != nil || err4 != nil {
		return errors.New("Failed to upload one of the machine jsons.")
	}
	return nil
}

func genFieldSchema(resourceFieldStruct ResourceFieldConfigs, field, fieldType, auth string) {
	resourceFieldStruct[field] = ResourceFieldConfig{
		Nullable: true,
		Type:     fieldType,
		Create:   strings.Contains(auth, "c"),
		Update:   strings.Contains(auth, "u"),
	}
}
func nameField(resourceFieldStruct ResourceFieldConfigs) {
	resourceFieldStruct["name"] = ResourceFieldConfig{
		Type:     "string",
		Create:   true,
		Update:   false,
		Required: true,
		Nullable: false,
	}
}

func uploadMachineServiceJSON(drivers []string) error {
	resourceFieldStruct := make(map[string]interface{})
	resourceFieldMap := make(ResourceFieldConfigs)
	resourceFieldStruct["collectionMethods"] = []string{"GET", "POST", "PUT", "DELETE"}
	resourceFieldStruct["resourceMethods"] = []string{"GET", "PUT", "DELETE"}
	resourceFieldStruct["resourceFields"] = resourceFieldMap
	for _, driver := range drivers {
		genFieldSchema(resourceFieldMap, driver+"Config", driver+"Config", "c")
	}
	genFieldSchema(resourceFieldMap, "authCertificateAuthority", "string", "c")
	genFieldSchema(resourceFieldMap, "authKey", "string", "c")
	genFieldSchema(resourceFieldMap, "dockerVersion", "string", "c")
	genFieldSchema(resourceFieldMap, "engineEnv", "map[string]", "c")
	genFieldSchema(resourceFieldMap, "engineInsecureRegistry", "array[string]", "c")
	genFieldSchema(resourceFieldMap, "engineInstallUrl", "string", "c")
	genFieldSchema(resourceFieldMap, "engineLabel", "map[string]", "c")
	genFieldSchema(resourceFieldMap, "engineOpt", "map[string]", "c")
	genFieldSchema(resourceFieldMap, "engineRegistryMirror", "array[string]", "c")
	genFieldSchema(resourceFieldMap, "engineStorageDriver", "string", "c")
	genFieldSchema(resourceFieldMap, "extractedConfig", "string", "u")
	genFieldSchema(resourceFieldMap, "labels", "map[string]", "cu")
	nameField(resourceFieldMap)

	jsonData, err := json.MarshalIndent(resourceFieldStruct, "", "    ")
	if err != nil {
		return err
	}
	return uploadDynamicSchema("machine", string(jsonData), "physicalHost", []string{"service"}, true)
}

func uploadMachineProjectJSON(drivers []string) error {
	resourceFieldStruct := make(map[string]interface{})
	resourceFieldMap := make(ResourceFieldConfigs)
	resourceFieldStruct["collectionMethods"] = []string{"GET", "POST", "DELETE"}
	resourceFieldStruct["resourceMethods"] = []string{"GET", "DELETE"}
	resourceFieldStruct["resourceFields"] = resourceFieldMap
	for _, driver := range drivers {
		genFieldSchema(resourceFieldMap, driver+"Config", driver+"Config", "c")
	}
	genFieldSchema(resourceFieldMap, "authCertificateAuthority", "string", "c")
	genFieldSchema(resourceFieldMap, "authKey", "string", "c")
	genFieldSchema(resourceFieldMap, "dockerVersion", "string", "c")
	genFieldSchema(resourceFieldMap, "engineEnv", "map[string]", "c")
	genFieldSchema(resourceFieldMap, "engineInsecureRegistry", "array[string]", "c")
	genFieldSchema(resourceFieldMap, "engineInstallUrl", "string", "c")
	genFieldSchema(resourceFieldMap, "engineLabel", "map[string]", "c")
	genFieldSchema(resourceFieldMap, "engineOpt", "map[string]", "c")
	genFieldSchema(resourceFieldMap, "engineRegistryMirror", "array[string]", "c")
	genFieldSchema(resourceFieldMap, "engineStorageDriver", "string", "c")
	genFieldSchema(resourceFieldMap, "labels", "map[string]", "c")
	nameField(resourceFieldMap)

	jsonData, err := json.MarshalIndent(resourceFieldStruct, "", "    ")
	if err != nil {
		return err
	}
	return uploadDynamicSchema("machine", string(jsonData), "physicalHost",
		[]string{"project", "member", "owner"}, false)
}

func uploadMachineUserJSON(drivers []string) error {
	resourceFieldStruct := make(map[string]interface{})
	resourceFieldMap := make(ResourceFieldConfigs)
	resourceFieldStruct["collectionMethods"] = []string{"GET"}
	resourceFieldStruct["resourceMethods"] = []string{"GET"}
	resourceFieldStruct["resourceFields"] = resourceFieldMap
	for _, driver := range drivers {
		genFieldSchema(resourceFieldMap, driver+"Config", driver+"Config", "")
	}
	genFieldSchema(resourceFieldMap, "authCertificateAuthority", "string", "")
	genFieldSchema(resourceFieldMap, "authKey", "string", "")
	genFieldSchema(resourceFieldMap, "dockerVersion", "string", "")
	genFieldSchema(resourceFieldMap, "driver", "string", "")
	genFieldSchema(resourceFieldMap, "engineEnv", "map[string]", "")
	genFieldSchema(resourceFieldMap, "engineInsecureRegistry", "array[string]", "")
	genFieldSchema(resourceFieldMap, "engineInstallUrl", "string", "")
	genFieldSchema(resourceFieldMap, "engineLabel", "map[string]", "")
	genFieldSchema(resourceFieldMap, "engineOpt", "map[string]", "")
	genFieldSchema(resourceFieldMap, "engineRegistryMirror", "array[string]", "")
	genFieldSchema(resourceFieldMap, "engineStorageDriver", "string", "")
	genFieldSchema(resourceFieldMap, "labels", "map[string]", "")
	nameField(resourceFieldMap)

	jsonData, err := json.MarshalIndent(resourceFieldStruct, "", "    ")
	if err != nil {
		return err
	}
	return uploadDynamicSchema("machine", string(jsonData), "physicalHost", []string{"admin", "user", "readAdmin"},
		false)
}

func uploadMachineReadOnlyJSON(drivers []string) error {
	resourceFieldStruct := make(map[string]interface{})
	resourceFieldMap := make(ResourceFieldConfigs)
	resourceFieldStruct["collectionMethods"] = []string{"GET"}
	resourceFieldStruct["resourceMethods"] = []string{"GET"}
	resourceFieldStruct["resourceFields"] = resourceFieldMap
	genFieldSchema(resourceFieldMap, "authCertificateAuthority", "string", "")
	genFieldSchema(resourceFieldMap, "authKey", "string", "")
	genFieldSchema(resourceFieldMap, "dockerVersion", "string", "")
	genFieldSchema(resourceFieldMap, "driver", "string", "")
	genFieldSchema(resourceFieldMap, "engineEnv", "map[string]", "")
	genFieldSchema(resourceFieldMap, "engineInsecureRegistry", "array[string]", "")
	genFieldSchema(resourceFieldMap, "engineInstallUrl", "string", "")
	genFieldSchema(resourceFieldMap, "engineLabel", "map[string]", "")
	genFieldSchema(resourceFieldMap, "engineOpt", "map[string]", "")
	genFieldSchema(resourceFieldMap, "engineRegistryMirror", "array[string]", "")
	genFieldSchema(resourceFieldMap, "engineStorageDriver", "string", "")
	genFieldSchema(resourceFieldMap, "labels", "map[string]", "")
	nameField(resourceFieldMap)

	jsonData, err := json.MarshalIndent(resourceFieldStruct, "", "    ")
	if err != nil {
		return err
	}
	return uploadDynamicSchema("machine", string(jsonData), "physicalHost", []string{"readonly"},
		false)
}
