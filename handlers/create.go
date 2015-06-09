package handlers

import (
	"bufio"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-machine-service/handlers/providers"
	"github.com/rancherio/go-rancher/client"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"
)

const (
	levelInfo            = "level=\"info\""
	levelError           = "level=\"error\""
	errorCreatingMachine = "Error creating machine: "
	CREATED_FILE         = "created"
)

var regExDockerMsg = regexp.MustCompile("msg=.*")
var regExHyphen = regexp.MustCompile("([a-z])([A-Z])")

func CreateMachine(event *events.Event, apiClient *client.RancherClient) error {
	log.WithFields(log.Fields{
		"resourceId": event.ResourceId,
		"eventId":    event.Id,
	}).Info("Creating Machine")

	machine, err := getMachine(event.ResourceId, apiClient)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	// Idempotency. If the resource has the property, we're done.
	if _, ok := machine.Data[machineDirField]; ok {
		reply := newReply(event)
		return publishReply(reply, apiClient)
	}

	machineDir, err := buildBaseMachineDir(machine.ExternalId)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	dataUpdates := map[string]interface{}{machineDirField: machineDir}
	eventDataWrapper := map[string]interface{}{"+data": dataUpdates}

	//Idempotency, if the same request is sent, without the machineDir & extractedConfig Field, we need to handle that
	if _, err := os.Stat(filepath.Join(machineDir, "machines", machine.Name, CREATED_FILE)); !os.IsNotExist(err) {
		extractedConfig, extractionErr := getIdempotentExtractedConfig(machine, machineDir, apiClient)
		if extractionErr != nil {
			return handleByIdError(extractionErr, event, apiClient)
		}
		dataUpdates["+fields"] = map[string]interface{}{"extractedConfig": extractedConfig}
		reply := newReply(event)
		reply.Data = eventDataWrapper
		return publishReply(reply, apiClient)
	}

	if providerHandler := providers.GetProviderHandler(machine.Driver); providerHandler != nil {
		if err := providerHandler(machine, machineDir); err != nil {
			return handleByIdError(err, event, apiClient)
		}
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
	go logProgress(readerStdout, readerStderr, publishChan, machine, event, errChan)

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
		"resourceId":        event.ResourceId,
		"machineExternalId": machine.ExternalId,
	}).Info("Machine Created")

	if f, err := os.Create(filepath.Join(machineDir, "machines", machine.Name, CREATED_FILE)); err != nil {
		return err
	} else {
		f.Close()
	}

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

func logProgress(readerStdout io.Reader, readerStderr io.Reader, publishChan chan<- string, machine *client.Machine, event *events.Event, errChan chan<- string) {
	// We will just log stdout first, then stderr, ignoring all errors.
	defer close(errChan)
	scanner := bufio.NewScanner(readerStdout)
	for scanner.Scan() {
		msg := scanner.Text()
		log.WithFields(log.Fields{
			"resourceId: ": event.ResourceId,
		}).Infof("stdout: %s", msg)
		msg = filterDockerMessage(msg, machine, errChan)
		if msg != "" {
			publishChan <- msg
		}
	}
	scanner = bufio.NewScanner(readerStderr)
	for scanner.Scan() {
		log.WithFields(log.Fields{
			"resourceId": event.ResourceId,
		}).Infof("stderr: %s", scanner.Text())
	}
}

func filterDockerMessage(msg string, machine *client.Machine, errChan chan<- string) string {
	// Docker log messages come in the format: time=<t> level=<log-level> msg=<message>
	// The minimum string should be greater than 7 characters msg="."
	match := regExDockerMsg.FindString(msg)
	if len(match) < 7 || !strings.HasPrefix(match, "msg=\"") {
		return ""
	}

	match = (match[5 : len(strings.TrimSpace(match))-1])
	if strings.Contains(msg, levelInfo) {
		// We just want to return <message> to cattle and only messages that do not contain the machine uuid or namee
		if strings.Contains(msg, machine.ExternalId) || strings.Contains(msg, machine.Name) {
			return ""
		}
		return match
	} else if strings.Contains(msg, levelError) {
		errChan <- strings.Replace(match, errorCreatingMachine, "", 1)
		return ""
	}
	return ""
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

	valueOfMachine := reflect.ValueOf(machine).Elem()

	// Grab the reflected Value of XyzConfig (i.e. DigitaloceanConfig) based on the machine driver
	driverConfig := valueOfMachine.FieldByName(strings.ToUpper(sDriver[:1]) + sDriver[1:] + "Config").Elem()
	typeOfDriverConfig := driverConfig.Type()

	for i := 0; i < driverConfig.NumField(); i++ {
		// We are ignoring the Resource Field as we don't need it.
		nameConfigField := typeOfDriverConfig.Field(i).Name
		if nameConfigField == "Resource" {
			continue
		}

		f := driverConfig.Field(i)

		// This converts all field name of ParameterName to --<driver name>-parameter-name
		// i.e. AccessToken parameter for DigitalOcean driver becomes --digitalocean-access-token
		dmField := "--" + sDriver + "-" + strings.ToLower(regExHyphen.ReplaceAllString(nameConfigField, "${1}-${2}"))

		// For now, we only support bool and string.  Will add more as required.
		switch f.Interface().(type) {
		case bool:
			// dm only accepts field or field=true if value=true
			if f.Bool() {
				cmd = append(cmd, dmField)
			}
		case string:
			if f.String() != "" {
				cmd = append(cmd, dmField, f.String())
			}
		default:
			return nil, fmt.Errorf("Unsupported type: %v", f.Type())
		}

	}

	cmd = append(cmd, machine.Name)
	log.Infof("Cmd slice: %v", cmd)
	return cmd, nil
}
