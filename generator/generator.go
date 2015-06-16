// +build ignore

package main

import (
	"fmt"
	"os"
	"strings"

	_ "github.com/docker/machine/drivers/amazonec2"
	_ "github.com/docker/machine/drivers/azure"
	_ "github.com/docker/machine/drivers/digitalocean"
	_ "github.com/docker/machine/drivers/exoscale"
	//	_ "github.com/docker/machine/drivers/google"
	_ "github.com/docker/machine/drivers/hyperv"
	_ "github.com/docker/machine/drivers/packet"
	_ "github.com/docker/machine/drivers/rackspace"
	_ "github.com/docker/machine/drivers/softlayer"
	_ "github.com/docker/machine/drivers/virtualbox"
	_ "github.com/docker/machine/drivers/vmwarefusion"
	_ "github.com/docker/machine/drivers/vmwarevcloudair"
	_ "github.com/docker/machine/drivers/vmwarevsphere"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/machine/drivers"
	"github.com/rancherio/go-machine-service/generator/helpers"
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

func fetchData() (*helpers.ResourceData, error) {
	blacklist := loadBlacklist()
	resourceMap := make(map[string]helpers.ResourceFields)
	documentationMap := make(map[string][]helpers.DocumentationFields)
	for _, driver := range drivers.GetDriverNames() {
		resourceFieldStruct := make(helpers.ResourceFields)
		resourceFieldMap := make(helpers.ResourceFieldConfigs)
		resourceFieldStruct["resourceFields"] = resourceFieldMap
		resourceMap[driver] = resourceFieldStruct
		documentationFieldStruct := new(helpers.DocumentationFields)
		documentationFieldMap := make(helpers.DocumentationFieldConfigs)
		documentationFieldStruct.ResourceFields = documentationFieldMap
		documentationFieldStruct.Id = driver + "Config"
		documentationMap[driver] = []helpers.DocumentationFields{*documentationFieldStruct}
		createFlags, err := drivers.GetCreateFlagsForDriver(driver)
		if err != nil {
			return nil, fmt.Errorf("error getting create flags for driver=%s, err=%v", driver, err)
		}
		for _, flagStruct := range createFlags {
			var flagName, flagType, description string
			var err error
			switch flagStruct.(type) {
			case cli.StringFlag:
				flag := flagStruct.(cli.StringFlag)
				flagName, err = getRancherName(flag.Name)
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "string"
				description = flag.Usage
			case cli.IntFlag:
				flag := flagStruct.(cli.IntFlag)
				flagName, err = getRancherName(flag.Name)
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "string"
				description = flag.Usage
			case cli.BoolFlag:
				flag := flagStruct.(cli.BoolFlag)
				flagName, err = getRancherName(flag.Name)
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "boolean"
				description = flag.Usage
			case cli.BoolTFlag:
				flag := flagStruct.(cli.BoolTFlag)
				flagName, err = getRancherName(flag.Name)
				if err != nil {
					return nil, fmt.Errorf("error getting the rancher name of flagStruct=%v for driver=%s, err=%v", flagStruct, driver, err)
				}
				flagType = "boolean"
				description = flag.Usage
			default:
				return nil, fmt.Errorf("unknown type of flag %v, for driver=%s", flagStruct, driver)
			}
			documentationFieldMap[flagName] = helpers.DocumentationFieldConfig{Description: description}
			resourceFieldMap[flagName] = helpers.ResourceFieldConfig{Type: flagType, Nullable: true, Required: false}
		}
	}
	return &helpers.ResourceData{Blacklist: blacklist, Drivers: drivers.GetDriverNames(), ResourceMap: resourceMap, DocumentationMap: documentationMap}, nil
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
