package dynamicDrivers

import (
	"fmt"
	"reflect"
	"strings"
	"unsafe"

	log "github.com/Sirupsen/logrus"
	cli "github.com/docker/machine/libmachine/mcnflag"

	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	rpcdriver "github.com/docker/machine/libmachine/drivers/rpc"
	"github.com/rancher/go-rancher/client"
	"net/rpc"

	"encoding/json"
	"errors"
)

type CreateFlag struct {
	Name        string      `json:"name,omitempty"`
	Type        string      `json:"type,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Description string      `json:"createFlag.Description,omitempty"`
	Create      bool        `json:"create,omitempty"`
}

func newCreateFlag(flag cli.Flag) (*CreateFlag, error) {
	createFlag := &CreateFlag{
		Default: flag.Default(),
		Create:  true,
	}
	var err error
	flagValue := reflect.ValueOf(flag)
	flagPointer := flagValue.Pointer()
	switch flagValue.Type().String() {
	case "*mcnflag.StringFlag":
		createFlag.Name, err = getRancherName(flag.String())
		if err != nil {
			return nil, fmt.Errorf("error getting the rancher name of flag=%v err=%v", flag, err)
		}
		createFlag.Type = "string"
		stringFlag := (*cli.StringFlag)(unsafe.Pointer(flagPointer))
		createFlag.Description = stringFlag.Usage
	case "*mcnflag.IntFlag":
		createFlag.Name, err = getRancherName(flag.String())
		if err != nil {
			return nil, fmt.Errorf("error getting the rancher name of flag=%v err=%v", flag, err)
		}
		createFlag.Type = "string"
		intFlag := (*cli.IntFlag)(unsafe.Pointer(flagPointer))
		createFlag.Description = intFlag.Usage
	case "*mcnflag.BoolFlag":
		createFlag.Name, err = getRancherName(flag.String())
		if err != nil {
			return nil, fmt.Errorf("error getting the rancher name of flag=%v err=%v", flag, err)
		}
		createFlag.Type = "boolean"
		booleanFlag := (*cli.BoolFlag)(unsafe.Pointer(flagPointer))
		createFlag.Description = booleanFlag.Usage
	case "*mcnflag.StringSliceFlag":
		createFlag.Name, err = getRancherName(flag.String())
		if err != nil {
			return nil, fmt.Errorf("error getting the rancher name of flag=%v err=%v", flag, err)
		}
		createFlag.Type = "array[string]"
		stringSliceFlag := (*cli.StringSliceFlag)(unsafe.Pointer(flagPointer))
		createFlag.Description = stringSliceFlag.Usage
	default:
		return nil, fmt.Errorf("unknown type of flag %v", flag)
	}
	return createFlag, nil
}

func getRancherName(machineFlagName string) (string, error) {
	parts := strings.SplitN(machineFlagName, "-", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("parameter %s does not follow expected naming convention [DRIVER]-[FLAG-NAME]", machineFlagName)
	}
	flagNameParts := strings.Split(parts[1], "-")
	flagName := flagNameParts[0]
	for i, flagNamePart := range flagNameParts {
		if i == 0 {
			continue
		}
		flagName = flagName + strings.ToUpper(flagNamePart[:1]) + flagNamePart[1:]
	}
	return flagName, nil
}

func generateAndUploadSchema(driver string) error {
	driverName := strings.TrimPrefix(driver, "docker-machine-driver-")
	flags, err := getCreateFlagsForDriver(driverName)
	if err != nil {
		return err
	}

	var createFlags []CreateFlag

	for _, fl := range flags {
		cFlag, err := newCreateFlag(fl)
		if err != nil {
			return nil
		}
		createFlags = append(createFlags, *cFlag)
	}

	json, err := flagsToJSON(createFlags)
	if err != nil {
		return err
	}
	return uploadDynamicSchema(driverName+"Config", json, "baseMachineConfig", []string{"service", "member",
		"owner", "project", "admin", "user", "readAdmin", "readonly", "restricted"}, true)
}

func removeSchema(schemaName string, apiClient *client.RancherClient) error {
	listOpts := client.NewListOpts()
	listOpts.Filters["name"] = schemaName
	listOpts.Filters["limit"] = "-1"
	listOpts.Filters["state_ne"] = "purged"
	schemas, err := apiClient.DynamicSchema.List(listOpts)
	if err != nil {
		return err
	}

	if len(schemas.Data) > 0 {
		log.Debugf("Removing %d schemas for %s", len(schemas.Data), schemaName)
		for _, schema := range schemas.Data {
			log.Debugf("Removing ", schemaName, " Id: ", schema.Id, " state: ", schema.State)
			if schema.State == "creating" {
				err = waitSuccessSchema(schema, apiClient)
				if err != nil {
					return err
				}
			}
			_, err = apiClient.DynamicSchema.ActionRemove(&schema)
			if err != nil {
				return err
			}
			err = waitSuccessSchema(schema, apiClient)
			if err != nil {
				return err
			}
			log.Debug("Removed ", schemaName, " Id: ", schema.Id)
		}
	}
	return nil
}

func uploadDynamicSchema(schemaName, definition, parent string, roles []string, delete bool) error {
	apiClient, err := getClient()
	if err != nil {
		return err
	}
	if delete {
		removeSchema(schemaName, apiClient)
	}

	schema, err := apiClient.DynamicSchema.Create(&client.DynamicSchema{
		Definition: definition,
		Name:       schemaName,
		Parent:     parent,
		Roles:      roles,
	})
	if err != nil {
		return errors.New(fmt.Sprint("Failed when uploading ", schemaName, " schema to cattle: ", err.Error()))
	}
	log.Debugf("Waiting for schema: %v to activate.", schema.Name)
	return waitSuccessSchema(*schema, apiClient)
}

func flagsToJSON(createFlags []CreateFlag) (string, error) {
	resourceFieldStruct := make(map[string]interface{})
	resourceFieldMap := make(ResourceFieldConfigs)
	resourceFieldStruct["collectionMethods"] = []string{"GET", "POST"}
	resourceFieldStruct["resourceFields"] = resourceFieldMap
	for _, flag := range createFlags {
		resourceFieldMap[flag.Name] = flagToField(flag)
	}
	fieldsJSON, err := json.MarshalIndent(resourceFieldStruct, "", "    ")
	return string(fieldsJSON), err
}

func flagToField(flag CreateFlag) ResourceFieldConfig {
	return ResourceFieldConfig{
		Type:        flag.Type,
		Nullable:    true,
		Required:    false,
		Create:      flag.Create,
		Update:      true,
		Description: flag.Description,
	}
}

func getCreateFlagsForDriver(driver string) ([]cli.Flag, error) {
	log.Debug("Starting binary ", driver)
	p, err := localbinary.NewPlugin(driver)
	if err != nil {
		return nil, err
	}
	go func() {
		err := p.Serve()
		if err != nil {
			log.Debugf("Error serving plugin server for driver=%s, err=%v", driver, err)
		}
	}()
	defer p.Close()
	addr, err := p.Address()
	if err != nil {
		return nil, err
	}

	rpcclient, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("Error dialing to plugin server's address(%v), err=%v", addr, err)
	}
	defer rpcclient.Close()

	c := rpcdriver.NewInternalClient(rpcclient)

	var flags []cli.Flag

	if err := c.Call(".GetCreateFlags", struct{}{}, &flags); err != nil {
		return nil, fmt.Errorf("Error getting flags err=%v", err)
	}

	return flags, nil
}
