package handlers

import (
	"fmt"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
	"os"
	"os/exec"
	"regexp"
	"time"
)

const (
	machineDirEnvKey = "MACHINE_STORAGE_PATH="
	machineDirField  = "machineDir"
	machineCmd       = "docker-machine"
)

var RegExMachineDirEnv = regexp.MustCompile("^" + machineDirEnvKey + ".*")

func PingNoOp(event *events.Event, apiClient *client.RancherClient) error {
	// No-op ping handler
	return nil
}

func getMachineDir(machine *client.Machine) (string, error) {
	mDir, ok := machine.Data[machineDirField]
	if !ok {
		return "", fmt.Errorf("MachineDir field not available for machine [%v].", machine.Id)
	}
	machineDir := mDir.(string)
	return machineDir, nil
}

var publishReply = func(reply *client.Publish, apiClient *client.RancherClient) error {
	_, err := apiClient.Publish.Create(reply)
	return err
}

var publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {
	// Since this is only updating the msg for the state transition, we will ignore errors here
	replyT := newReply(event)
	replyT.Transitioning = "yes"
	replyT.TransitioningMessage = msg
	publishReply(replyT, apiClient)
}

func republishTransitioningReply(publishChan <-chan string, event *events.Event, apiClient *client.RancherClient) {
	// We only do this because there is a current issue within Cattle that if a transition message
	// has not been updated for a period of time, it can no longer be updated.  For now, to deal with this
	// we will simply republish transitioning messages until the next one is added.
	// Because this ticker is going to republish every X seconds, it's will most likely republish a message sooner
	// In all liklihood, we will remove this method later.
	defaultWaitTime := time.Second * 15
	ticker := time.NewTicker(defaultWaitTime)
	var lastMsg string
	for {
		select {
		case msg, more := <-publishChan:
			if !more {
				ticker.Stop()
				return
			} else {
				lastMsg = msg
				publishTransitioningReply(lastMsg, event, apiClient)
			}

		case <-ticker.C:
			//republish last message
			if lastMsg != "" {
				publishTransitioningReply(lastMsg, event, apiClient)
			}
		}
	}
}

var getMachine = func(id string, apiClient *client.RancherClient) (*client.Machine, error) {
	return apiClient.Machine.ById(id)
}

func handleByIdError(err error, event *events.Event, apiClient *client.RancherClient) error {
	apiError, ok := err.(*client.ApiError)
	if !ok || apiError.StatusCode != 404 {
		return err
	}
	// 404 Indicates this is a physicalHost but not a machine. Just reply.
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
