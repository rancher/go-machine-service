package handlers

import (
	"bufio"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-machine-service/handlers/providers"
	"github.com/rancher/go-rancher/client"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	levelInfo            = "level=\"info\""
	levelError           = "level=\"error\""
	errorCreatingMachine = "Error creating machine: "
	CreatedFile          = "created"
	MachineKind          = "machine"
)

var regExDockerMsg = regexp.MustCompile("msg=.*")
var regExHyphen = regexp.MustCompile("([a-z])([A-Z])")

func CreateMachine(event *events.Event, apiClient *client.RancherClient) (err error) {
	log.WithFields(log.Fields{
		"resourceId": event.ResourceID,
		"eventId":    event.ID,
	}).Info("Creating Machine")

	machine, err := getMachine(event.ResourceID, apiClient)
	if err != nil {
		return err
	}
	if machine == nil {
		return notAMachineReply(event, apiClient)
	}

	// Idempotency. If the resource has the property, we're done.
	if _, ok := machine.Data[machineDirField]; ok {
		reply := newReply(event)
		return publishReply(reply, apiClient)
	}

	machineDir, err := buildBaseMachineDir(machine.ExternalId)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			cleanupResources(machineDir, machine.Name)
		}
	}()

	dataUpdates := map[string]interface{}{machineDirField: machineDir}
	eventDataWrapper := map[string]interface{}{"+data": dataUpdates}

	//Idempotency, if the same request is sent, without the machineDir & extractedConfig Field, we need to handle that
	if _, err := os.Stat(filepath.Join(machineDir, "machines", machine.Name, CreatedFile)); !os.IsNotExist(err) {
		extractedConfig, extractionErr := getIdempotentExtractedConfig(machine, machineDir, apiClient)
		if extractionErr != nil {
			return extractionErr
		}
		dataUpdates["+fields"] = map[string]interface{}{"extractedConfig": extractedConfig}
		reply := newReply(event)
		reply.Data = eventDataWrapper
		return publishReply(reply, apiClient)
	}

	providerHandler := providers.GetProviderHandler(machine.Driver)

	if err := providerHandler.HandleCreate(machine, machineDir); err != nil {
		return err
	}

	command, err := buildCreateCommand(machine, machineDir)
	if err != nil {
		return err
	}

	//Setup republishing timer
	publishChan := make(chan string, 10)
	go republishTransitioningReply(publishChan, event, apiClient)

	publishChan <- "Contacting " + machine.Driver
	alreadyClosed := false
	defer func() {
		if !alreadyClosed {
			close(publishChan)
		}
	}()

	readerStdout, readerStderr, err := startReturnOutput(command)
	if err != nil {
		return err
	}

	errChan := make(chan string, 1)
	go logProgress(readerStdout, readerStderr, publishChan, machine, event, errChan, providerHandler)

	err = command.Wait()

	if err != nil {
		select {
		case errString := <-errChan:
			if errString != "" {
				return fmt.Errorf(errString)
			}
		case <-time.After(10 * time.Second):
			log.Error("Waited 10 seconds to break after command.Wait().  Please review logProgress.")
		}
		return err
	}

	log.WithFields(log.Fields{
		"resourceId":        event.ResourceID,
		"machineExternalId": machine.ExternalId,
	}).Info("Machine Created")

	f, err := os.Create(filepath.Join(machineDir, "machines", machine.Name, CreatedFile))
	if err != nil {
		return err
	}
	f.Close()

	destFile, err := createExtractedConfig(event, machine)
	if err != nil {
		return err
	}

	if destFile != "" {
		publishChan <- "Saving Machine Config"
		extractedConf, err := getExtractedConfig(destFile, machine, apiClient)
		if err != nil {
			return err
		}
		dataUpdates["+fields"] = map[string]string{"extractedConfig": extractedConf}
	}

	reply := newReply(event)
	reply.Data = eventDataWrapper

	// Explicitly close publish channel so that it doesn't conflict with final reply
	close(publishChan)
	alreadyClosed = true
	return publishReply(reply, apiClient)
}

func logProgress(readerStdout io.Reader, readerStderr io.Reader, publishChan chan<- string, machine *client.Machine, event *events.Event, errChan chan<- string, providerHandler providers.Provider) {
	// We will just log stdout first, then stderr, ignoring all errors.
	defer close(errChan)
	scanner := bufio.NewScanner(readerStdout)
	for scanner.Scan() {
		msg := scanner.Text()
		log.WithFields(log.Fields{
			"resourceId: ": event.ResourceID,
		}).Infof("stdout: %s", msg)
		transitionMsg := filterDockerMessage(msg, machine, errChan, providerHandler, false)
		if transitionMsg != "" {
			publishChan <- transitionMsg
		}
	}
	scanner = bufio.NewScanner(readerStderr)
	for scanner.Scan() {
		msg := scanner.Text()
		log.WithFields(log.Fields{
			"resourceId": event.ResourceID,
		}).Infof("stderr: %s", msg)
		filterDockerMessage(msg, machine, errChan, providerHandler, true)
	}
}

func filterDockerMessage(msg string, machine *client.Machine, errChan chan<- string, providerHandler providers.Provider, errMsg bool) string {
	if strings.Contains(msg, errorCreatingMachine) || errMsg {
		errChan <- providerHandler.HandleError(strings.Replace(msg, errorCreatingMachine, "", 1))
		return ""
	}
	if strings.Contains(msg, machine.ExternalId) || strings.Contains(msg, machine.Name) {
		return ""
	}
	return msg
}

func startReturnOutput(command *exec.Cmd) (io.Reader, io.Reader, error) {
	readerStdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	readerStderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	err = command.Start()
	if err != nil {

		defer readerStdout.Close()
		defer readerStderr.Close()
		return nil, nil, err
	}
	return readerStdout, readerStderr, nil
}

func buildCreateCommand(machine *client.Machine, machineDir string) (*exec.Cmd, error) {
	cmdArgs, err := buildMachineCreateCmd(machine)
	if err != nil {
		return nil, err
	}

	command := buildCommand(machineDir, cmdArgs)
	return command, nil
}

func buildMachineCreateCmd(machine *client.Machine) ([]string, error) {
	sDriver := strings.ToLower(machine.Driver)
	cmd := []string{"create", "-d", sDriver}

	cmd = append(cmd, buildEngineOpts("--engine-install-url", []string{machine.EngineInstallUrl})...)
	cmd = append(cmd, buildEngineOpts("--engine-opt", mapToSlice(machine.EngineOpt))...)
	cmd = append(cmd, buildEngineOpts("--engine-env", mapToSlice(machine.EngineEnv))...)
	cmd = append(cmd, buildEngineOpts("--engine-insecure-registry", machine.EngineInsecureRegistry)...)
	cmd = append(cmd, buildEngineOpts("--engine-label", mapToSlice(machine.EngineLabel))...)
	cmd = append(cmd, buildEngineOpts("--engine-registry-mirror", machine.EngineRegistryMirror)...)
	cmd = append(cmd, buildEngineOpts("--engine-storage-driver", []string{machine.EngineStorageDriver})...)

	// Grab the reflected Value of XyzConfig (i.e. DigitaloceanConfig) based on the machine driver
	driverConfig := machine.Data["fields"].(map[string]interface{})[machine.Driver+"Config"]
	if driverConfig == nil {
		return nil, errors.New(machine.Driver + "Config does not exist on Machine " + machine.Id)
	}
	configFields := []string{}
	for k := range driverConfig.(map[string]interface{}) {
		configFields = append(configFields, k)
	}
	sort.Strings(configFields)
	driverMapConfig := driverConfig.(map[string]interface{})
	for _, nameConfigField := range configFields {
		// We are ignoring the Resource Field as we don't need it.
		if nameConfigField == "Resource" {
			continue
		}

		// This converts all field name of ParameterName to --<driver name>-parameter-name
		// i.e. AccessToken parameter for DigitalOcean driver becomes --digitalocean-access-token
		dmField := "--" + sDriver + "-" + strings.ToLower(regExHyphen.ReplaceAllString(nameConfigField, "${1}-${2}"))

		// For now, we only support bool and string.  Will add more as required.
		switch f := driverMapConfig[nameConfigField].(type) {
		case bool:
			// dm only accepts field or field=true if value=true
			if f {
				cmd = append(cmd, dmField)
			}
		case string:
			if f != "" {
				cmd = append(cmd, dmField, f)
			}
		case []string:
			for _, q := range f {
				cmd = append(cmd, dmField, q)
			}
		case []interface{}:
			for _, q := range f {
				cmd = append(cmd, dmField, fmt.Sprintf("%v", q))
			}
		case nil:
		default:
			return nil, fmt.Errorf("Unsupported type: %v", reflect.TypeOf(f))
		}

	}

	cmd = append(cmd, machine.Name)
	log.Infof("Cmd slice: %v", cmd)
	return cmd, nil
}

func mapToSlice(m map[string]interface{}) []string {
	ret := []string{}
	for k, v := range m {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func buildEngineOpts(name string, values []string) []string {
	opts := []string{}
	for _, value := range values {
		if value == "" {
			continue
		}
		opts = append(opts, name, value)
	}
	return opts
}
