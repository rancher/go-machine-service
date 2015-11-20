package handlers

import (
	"fmt"
	"net/rpc"
	"reflect"
	"strings"
	"unsafe"

	log "github.com/Sirupsen/logrus"
	rpcdriver "github.com/docker/machine/libmachine/drivers/rpc"
	cli "github.com/docker/machine/libmachine/mcnflag"

	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
)

type CreateFlag struct {
	Name        string      `json: name, "omitempty"`
	Type        string      `json: type, "omitempty"`
	Default     interface{} `json: default, "omitempty"`
	Description string      `json: createFlag.Description, "omitempty"`
}

func newCreateFlag(flag cli.Flag) (*CreateFlag, error) {
	createFlag := &CreateFlag{
		Default: flag.Default(),
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

func DriverInfo(event *events.Event, client *client.RancherClient) error {
	log.WithFields(log.Fields{
		"resourceId": event.ResourceId,
		"eventId":    event.Id,
	}).Info("Obtaining Driver Info")

	driverName := "" //obtain driver name

	p, err := localbinary.NewPlugin(driverName)
	if err != nil {
		return err
	}
	defer p.Close()

	go func() {
		if err := p.Serve(); err != nil {
			log.Errorf("Error running plugin server [%v]", err)
		}
	}()

	addr, err := p.Address()
	if err != nil {
		return err
	}

	rpcclient, err := rpc.DialHTTP("tcp", addr)
	if err != nil {
		return fmt.Errorf("Error dialing to plugin server's address(%v), err=%v", addr, err)
	}

	c := rpcdriver.NewInternalClient(rpcclient)

	var flags []cli.Flag

	if err := c.Call("RPCServerDriver.GetCreateFlags", struct{}{}, &flags); err != nil {
		return fmt.Errorf("Error getting flags err=%v", err)
	}

	var createFlags []CreateFlag

	for _, fl := range flags {
		cFlag, err := newCreateFlag(fl)
		if err != nil {
			return nil
		}
		createFlags = append(createFlags, *cFlag)
	}

	log.Errorf("create Flags = %+v", createFlags)

	return nil
}
