package handlers

import (
	"fmt"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
	"os"
	"os/exec"
	"regexp"
)

const (
	machineDirEnvKey = "MACHINE_DIR="
	machineDirField  = "machineDir"
	machineCmd       = "machine"
)

var RegExMachineDirEnv = regexp.MustCompile("^" + machineDirEnvKey + ".*")

func PingNoOp(event *events.Event, apiClient *client.RancherClient) error {
	// No-op ping handler
	return nil
}

func getMachineDir(physHost *client.MachineHost) (string, error) {
	mDir, ok := physHost.Data[machineDirField]
	if !ok {
		return "", fmt.Errorf("MachineDir field not available for machine [%v].", physHost.Id)
	}
	machineDir := mDir.(string)
	return machineDir, nil
}

func updateMachineData(machine *client.MachineHost, dataUpdates map[string]string,
	apiClient *client.RancherClient) error {
	latest, err := getMachine(machine.Id, apiClient)
	if err != nil {
		return err
	}
	data := latest.Data
	if data == nil {
		data = map[string]interface{}{}
	}
	for k, v := range dataUpdates {
		data[k] = v
	}
	return doMachineUpdate(latest, &client.MachineHost{Data: data}, apiClient)
}

var doMachineUpdate = func(current *client.MachineHost, machineUpdates *client.MachineHost,
	apiClient *client.RancherClient) error {
	_, err := apiClient.MachineHost.Update(current, machineUpdates)
	if err != nil {
		return err
	}
	return nil
}

var publishReply = func(reply *client.Publish, apiClient *client.RancherClient) error {
	_, err := apiClient.Publish.Create(reply)
	return err
}

var getMachine = func(id string, apiClient *client.RancherClient) (*client.MachineHost, error) {
	return apiClient.MachineHost.ById(id)
}

func handleByIdError(err error, event *events.Event, apiClient *client.RancherClient) error {
	apiError, ok := err.(*client.ApiError)
	if !ok || apiError.StatusCode != 404 {
		return err
	}
	// 404 Indicates this is a physicalHost but not a machineHost. Just reply.
	reply := newReply(event)
	return publishReply(reply, apiClient)
}

func newReply(event *events.Event) *client.Publish {
	return &client.Publish{
		Name:        event.ReplyTo,
		PreviousIds: []string{event.Id},
	}
}

func buildCommand(machineDir string, cmdArgs []string) *exec.Cmd {
	command := exec.Command(machineCmd, cmdArgs...)
	env := initEnviron(machineDir)
	command.Env = env
	return command
}

func initEnviron(machineDir string) []string {
	env := os.Environ()
	found := false
	for idx, ev := range env {
		if RegExMachineDirEnv.MatchString(ev) {
			env[idx] = machineDirEnvKey + machineDir
			found = true
		}
	}
	if !found {
		env = append(env, machineDirEnvKey+machineDir)
	}
	return env
}
