package dynamic

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-rancher/client"
)

func UploadMachineSchemas(apiClient *client.RancherClient, drivers ...string) error {
	schemaLock.Lock()
	defer schemaLock.Unlock()

	existing, err := apiClient.MachineDriver.List(&client.ListOpts{
		Filters: map[string]interface{}{
			"removed_null": "true",
		},
	})
	if err != nil {
		return err
	}

	for _, driver := range existing.Data {
		if driver.Name != "" && (driver.State == "active" || driver.State == "activating") {
			drivers = append(drivers, driver.Name)
		}
	}

	log.Infof("Updating machine jsons for  %v", drivers)
	if err := uploadMachineServiceJSON(drivers, true); err != nil {
		return err
	}
	if err := uploadMachineProjectJSON(drivers, false); err != nil {
		return err
	}
	if err := uploadMachineUserJSON(drivers, false); err != nil {
		return err
	}
	return uploadMachineReadOnlyJSON(false)
}

func field(resourceFields map[string]client.Field, field, fieldType, auth string) {
	resourceFields[field] = client.Field{
		Nullable: true,
		Type:     fieldType,
		Create:   strings.Contains(auth, "c"),
		Update:   strings.Contains(auth, "u"),
	}
}

func baseSchema(drivers []string, defaultAuth string) client.Schema {
	schema := client.Schema{
		CollectionMethods: []string{"GET"},
		ResourceMethods:   []string{"GET"},
		ResourceFields: map[string]client.Field{
			"name": client.Field{
				Type:     "string",
				Create:   true,
				Update:   false,
				Required: true,
				Nullable: false,
			},
		},
	}

	if strings.Contains(defaultAuth, "c") {
		schema.CollectionMethods = append(schema.CollectionMethods, "POST", "DELETE")
		schema.ResourceMethods = append(schema.ResourceMethods, "DELETE")
	}

	for _, driver := range drivers {
		field(schema.ResourceFields, driver+"Config", driver+"Config", defaultAuth)
	}

	field(schema.ResourceFields, "authCertificateAuthority", "string", defaultAuth)
	field(schema.ResourceFields, "authKey", "string", defaultAuth)
	field(schema.ResourceFields, "dockerVersion", "string", defaultAuth)
	field(schema.ResourceFields, "driver", "string", "")
	field(schema.ResourceFields, "engineEnv", "map[string]", defaultAuth)
	field(schema.ResourceFields, "engineInsecureRegistry", "array[string]", defaultAuth)
	field(schema.ResourceFields, "engineInstallUrl", "string", defaultAuth)
	field(schema.ResourceFields, "engineLabel", "map[string]", defaultAuth)
	field(schema.ResourceFields, "engineOpt", "map[string]", defaultAuth)
	field(schema.ResourceFields, "engineRegistryMirror", "array[string]", defaultAuth)
	field(schema.ResourceFields, "engineStorageDriver", "string", defaultAuth)
	field(schema.ResourceFields, "labels", "map[string]", defaultAuth)

	return schema
}

func uploadMachineServiceJSON(drivers []string, remove bool) error {
	schema := baseSchema(drivers, "c")
	schema.CollectionMethods = append(schema.CollectionMethods, "PUT")
	schema.ResourceMethods = append(schema.ResourceMethods, "PUT")
	field(schema.ResourceFields, "extractedConfig", "string", "u")
	field(schema.ResourceFields, "labels", "map[string]", "cu")

	return uploadMachineSchema(schema, []string{"service"}, remove)
}

func uploadMachineProjectJSON(drivers []string, remove bool) error {
	schema := baseSchema(drivers, "c")
	return uploadMachineSchema(schema, []string{"project", "member", "owner"}, remove)
}

func uploadMachineUserJSON(drivers []string, remove bool) error {
	schema := baseSchema(drivers, "")
	return uploadMachineSchema(schema, []string{"admin", "user", "readAdmin"}, remove)
}

func uploadMachineReadOnlyJSON(remove bool) error {
	schema := baseSchema([]string{}, "")
	return uploadMachineSchema(schema, []string{"readonly"}, remove)
}

func uploadMachineSchema(schema client.Schema, roles []string, remove bool) error {
	json, err := toJSON(&schema)
	if err != nil {
		return err
	}
	return uploadDynamicSchema("machine", json, "physicalHost", roles, remove)
}
