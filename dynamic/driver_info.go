package dynamic

import (
	"fmt"
	"net/rpc"
	"reflect"
	"strings"
	"sync"

	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	rpcdriver "github.com/docker/machine/libmachine/drivers/rpc"
	cli "github.com/docker/machine/libmachine/mcnflag"
	"github.com/rancher/go-rancher/v2"
)

const (
	schemaBase = "baseMachineConfig"
)

var (
	schemaLock  = sync.Mutex{}
	schemaRoles = []string{"service",
		"member",
		"owner",
		"project",
		"admin",
		"user",
		"readAdmin",
		"readonly",
		"restricted"}

	driverFields = map[string][]string{
		"aliyunecs":    []string{"sshKeypath"},
		"amazonec2":    []string{"keypairName", "sshKeypath", "userdata"},
		"azure":        []string{"customData"},
		"digitalocean": []string{"sshKeyPath", "userdata"},
		"ecl":          []string{"privateKeyFile"},
		"exoscale":     []string{"sshKey", "userdata"},
		"generic":      []string{"sshKey"},
		"hetzner":      []string{"existingKeyId", "existingKeyPath"},
		"openstack":    []string{"privateKeyFile"},
		"otc":          []string{"privateKeyFile"},
		"qingcloud":    []string{"sshKeypath"},
		"vultr":        []string{"userdata"},
	}
)

func flagToField(flag cli.Flag) (string, client.Field, error) {
	field := client.Field{
		Default: flag.Default(),
		Create:  true,
		Type:    "string",
	}

	name, err := toLowerCamelCase(flag.String())
	if err != nil {
		return name, field, err
	}

	switch v := flag.(type) {
	case *cli.StringFlag:
		field.Description = v.Usage
	case *cli.IntFlag:
		field.Description = v.Usage
	case *cli.BoolFlag:
		field.Type = "boolean"
		field.Description = v.Usage
	case *cli.StringSliceFlag:
		field.Type = "array[string]"
		field.Description = v.Usage
	default:
		return name, field, fmt.Errorf("unknown type of flag %v: %v", flag, reflect.TypeOf(flag))
	}

	if field.Type == "string" && field.Default != nil {
		field.Default = fmt.Sprintf("%v", field.Default)
	}

	return name, field, nil
}

func GenerateAndUploadSchema(driver string) error {
	schemaLock.Lock()
	defer schemaLock.Unlock()

	driverName := strings.TrimPrefix(driver, "docker-machine-driver-")
	flags, err := getCreateFlagsForDriver(driverName)
	if err != nil {
		return err
	}

	resourceFields := map[string]client.Field{}
	for _, flag := range flags {
		name, field, err := flagToField(flag)
		if err != nil {
			return err
		}
		resourceFields[name] = field
	}

	if fields, ok := driverFields[driverName]; ok {
		for _, field := range fields {
			delete(resourceFields, field)
		}
	}

	json, err := toJSON(&client.Schema{
		CollectionMethods: []string{"POST"},
		ResourceFields:    resourceFields,
	})
	if err != nil {
		return err
	}
	return uploadDynamicSchema(driverName+"Config", json, schemaBase, schemaRoles, true)
}

func RemoveSchemas(schemaName string, apiClient *client.RancherClient) error {
	listOpts := &client.ListOpts{
		Filters: map[string]interface{}{
			"name":         schemaName,
			"limit":        "-1",
			"removed_null": "true",
		},
	}
	schemas, err := apiClient.DynamicSchema.List(listOpts)
	if err != nil {
		return err
	}

	for _, schema := range schemas.Data {
		if err := waitSchema(schema, apiClient); err != nil {
			return err
		}

		if err := apiClient.Reload(&schema.Resource, &schema); err != nil {
			return err
		}

		if schema.Removed != "" {
			continue
		}

		logger.Debugf("Removing %s id: %s state: %s", schemaName, schema.Id, schema.State)
		if err := apiClient.DynamicSchema.Delete(&schema); err != nil {
			return err
		}

		if err := waitSchema(schema, apiClient); err != nil {
			return err
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
		RemoveSchemas(schemaName, apiClient)
	}

	schema, err := apiClient.DynamicSchema.Create(&client.DynamicSchema{
		Definition: definition,
		Name:       schemaName,
		Parent:     parent,
		Roles:      roles,
	})
	logger.WithField("id", schema.Id).Infof("Creating schema %s, roles %v", schemaName, roles)
	if err != nil {
		return fmt.Errorf("Failed when uploading %s schema: %v", schemaName, err)
	}

	return waitSchema(*schema, apiClient)
}

func getCreateFlagsForDriver(driver string) ([]cli.Flag, error) {
	logger.Debug("Starting binary ", driver)
	p, err := localbinary.NewPlugin(driver)
	if err != nil {
		return nil, err
	}
	go func() {
		err := p.Serve()
		if err != nil {
			logger.Debugf("Error serving plugin server for driver=%s, err=%v", driver, err)
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
