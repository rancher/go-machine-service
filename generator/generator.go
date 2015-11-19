// +build ignore

package main

import (
	"fmt"
	"net/rpc"
	"os"
	"reflect"
	"strings"
	"unsafe"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	rpcdriver "github.com/docker/machine/libmachine/drivers/rpc"
	cli "github.com/docker/machine/libmachine/mcnflag"
	"github.com/rancher/go-machine-service/generator/helpers"
)

func main() {
	err := generate()
	if err != nil {
		log.Fatal(err)
	}
}

func generate() error {
	resourceData, err := fetchData() // collects all data required to generate the files
	if err != nil {
		return fmt.Errorf("error collecting driver data err=%v", err)
	}
	err = helpers.GenerateAuthJsons(resourceData) // generate user_auth and project_auth jsons
	if err != nil {
		return fmt.Errorf("error generating auth jsons err=%v", err)
	}
	err = helpers.GenerateSpringContext(resourceData) // generate spring-docker-machine-api-context.xml
	if err != nil {
		return fmt.Errorf("error generating spring context err=%v", err)
	}
	err = helpers.GenerateFieldJsons(resourceData) // generate field information for each driver
	if err != nil {
		return fmt.Errorf("error generating field jsons err=%v", err)
	}
	err = helpers.GenerateDocJsons(resourceData) // generate doc information for each driver
	if err != nil {
		return fmt.Errorf("error generating doc jsons err=%v", err)
	}
	err = helpers.GenerateMachineJson(resourceData) // generate schema/base/machine.json.d/machine-generated.json
	if err != nil {
		return fmt.Errorf("error generating machine json err=%v", err)
	}
	return nil
}

func loadBlacklist() []string {
	blacklist := os.Getenv("RANCHER_DRIVER_BLACKLIST")
	if blacklist != "" {
		return strings.Split(blacklist, ",")
	}
	// defaulting to this list for now
	return []string{}
}

func getDriverNames() []string {
	return []string{"amazonec2", "azure", "digitalocean", "exoscale", "packet", "rackspace", "softlayer", "ubiquity", "virtualbox", "vmwarefusion", "vmwarevcloudair", "vmwarevsphere"}
}

func getCreateFlagsForDriver(driver string) ([]cli.Flag, error) {
	p, err := localbinary.NewPlugin(driver)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := p.Serve(); err != nil {
			log.Fatalf("Error starting plugin server for driver=%s, err=%v", driver, err)
		}
	}()

	addr, err := p.Address()
	if err != nil {
		panic("Error attempting to get plugin server address for RPC")
	}

	rpcclient, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("Error dialing to plugin server's address(%v), err=%v", addr, err)
	}

	c := rpcdriver.NewInternalClient(rpcclient)

	var flags []cli.Flag

	if err := c.Call("RPCServerDriver.GetCreateFlags", struct{}{}, &flags); err != nil {
		return nil, fmt.Errorf("Error getting flags err=%v", err)
	}

	return flags, nil
}

func fetchData() (*helpers.ResourceData, error) {
	blacklist := loadBlacklist()
	resourceMap := make(map[string]helpers.ResourceFields)
	documentationMap := make(map[string][]helpers.DocumentationFields)
	for _, driver := range getDriverNames() {
		resourceFieldStruct := make(helpers.ResourceFields)
		resourceFieldMap := make(helpers.ResourceFieldConfigs)
		resourceFieldStruct["resourceFields"] = resourceFieldMap
		resourceMap[driver] = resourceFieldStruct
		documentationFieldStruct := new(helpers.DocumentationFields)
		documentationFieldMap := make(helpers.DocumentationFieldConfigs)
		documentationFieldStruct.ResourceFields = documentationFieldMap
		documentationFieldStruct.Id = driver + "Config"
		documentationMap[driver] = []helpers.DocumentationFields{*documentationFieldStruct}
		createFlags, err := getCreateFlagsForDriver(driver)
		if err != nil {
			return nil, fmt.Errorf("error getting create flags for driver=%s, err=%v", driver, err)
		}
		for _, flagStruct := range createFlags {
			var flagName, flagType, description string
			var err error
			flagPointer := reflect.ValueOf(flagStruct).Pointer()
			switch reflect.TypeOf(flagStruct).String() {
			case "*mcnflag.StringFlag":
				flagName, err = getRancherName(flagStruct.String())
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "string"
				stringFlag := (*cli.StringFlag)(unsafe.Pointer(flagPointer))
				description = stringFlag.Usage
			case "*mcnflag.IntFlag":
				flagName, err = getRancherName(flagStruct.String())
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "string"
				intFlag := (*cli.IntFlag)(unsafe.Pointer(flagPointer))
				description = intFlag.Usage
			case "*mcnflag.BoolFlag":
				flagName, err = getRancherName(flagStruct.String())
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "boolean"
				booleanFlag := (*cli.BoolFlag)(unsafe.Pointer(flagPointer))
				description = booleanFlag.Usage
			case "*mcnflag.StringSliceFlag":
				flagName, err = getRancherName(flagStruct.String())
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "array[string]"
				stringSliceFlag := (*cli.StringSliceFlag)(unsafe.Pointer(flagPointer))
				description = stringSliceFlag.Usage
			default:
				return nil, fmt.Errorf("unknown type of flag %v, for driver=%s", flagStruct, driver)
			}
			documentationFieldMap[flagName] = helpers.DocumentationFieldConfig{Description: description}
			resourceFieldMap[flagName] = helpers.ResourceFieldConfig{Type: flagType, Nullable: true, Required: false}
		}
	}
	return &helpers.ResourceData{Blacklist: blacklist, Drivers: getDriverNames(), ResourceMap: resourceMap, DocumentationMap: documentationMap}, nil
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
