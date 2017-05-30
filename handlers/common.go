package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/rancher/event-subscriber/events"
	client "github.com/rancher/go-rancher/v2"
)

const (
	machineDirEnvKey  = "MACHINE_STORAGE_PATH="
	machineCmd        = "docker-machine"
	defaultCattleHome = "/var/lib/cattle"
)

func PingNoOp(event *events.Event, apiClient *client.RancherClient) error {
	// No-op ping handler
	return nil
}

func buildBaseMachineDir(m *client.Machine) (string, error) {
	machineDir := filepath.Join(getWorkDir(), "machines", m.ExternalId)
	return machineDir, os.MkdirAll(machineDir, 0740)
}

func getWorkDir() string {
	workDir := os.Getenv("MACHINE_WORK_DIR")
	if workDir == "" {
		workDir = os.Getenv("CATTLE_HOME")
	}
	if workDir == "" {
		workDir = defaultCattleHome
	}
	return filepath.Join(workDir, "machine")
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
	// In all likelihood, we will remove this method later.
	defaultWaitTime := time.Second * 15
	ticker := time.NewTicker(defaultWaitTime)
	var lastMsg string
	for {
		select {
		case msg, more := <-publishChan:
			if !more {
				ticker.Stop()
				return
			}
			lastMsg = msg
			publishTransitioningReply(lastMsg, event, apiClient)

		case <-ticker.C:
			//republish last message
			if lastMsg != "" {
				publishTransitioningReply(lastMsg, event, apiClient)
			}
		}
	}
}

var getMachine = func(id string, apiClient *client.RancherClient) (*client.Machine, error) {
	m, err := apiClient.Machine.ById(id)
	if err != nil || m == nil {
		return nil, err
	}

	host, err := getHost(m, apiClient)
	if err != nil || host == nil {
		return m, err
	}

	err = applyHostTemplate(host, m, apiClient)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to apply host template")
	}

	return m, nil
}

func applyHostTemplate(host *client.Host, m *client.Machine, apiClient *client.RancherClient) error {
	if host.HostTemplateId != "" {
		ht, err := apiClient.HostTemplate.ById(host.HostTemplateId)
		if err != nil {
			return err
		}
		return apply(m, ht, apiClient, true)
	}

	templates, err := apiClient.HostTemplate.List(&client.ListOpts{
		Filters: map[string]interface{}{
			"accountId":    m.AccountId,
			"driver":       m.Driver,
			"removed_null": "true",
			"state":        "active",
		},
	})
	if err != nil {
		return err
	}
	// If we find more than one we apply all secret values, but not public
	if len(templates.Data) > 0 {
		for _, ht := range templates.Data {
			if err := apply(m, &ht, apiClient, false); err != nil {
				return err
			}
		}
	} else if len(templates.Data) == 1 {
		return apply(m, &templates.Data[0], apiClient, true)
	}

	return nil
}

func apply(m *client.Machine, ht *client.HostTemplate, apiClient *client.RancherClient, public bool) error {
	if public {
		if err := copyData(m, ht.PublicValues); err != nil {
			return err
		}
	}

	secretValues := map[string]interface{}{}
	if err := apiClient.GetLink(ht.Resource, "secretValues", &secretValues); err != nil {
		return errors.Wrap(err, "Get secretValues link")
	}

	err := copyData(m, secretValues)
	if err != nil {
		return err
	}

	if err := populateFields(m); err != nil {
		return err
	}

	return err
}

func populateFields(m *client.Machine) error {
	content, err := json.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "populateFields marshall")
	}
	mm := map[string]interface{}{}
	if err := json.Unmarshal(content, &mm); err != nil {
		return errors.Wrap(err, "populateFields unmarshall to mm")
	}
	machineConfig := mm[m.Driver+"Config"]
	if machineConfig == nil {
		return nil
	}
	machineConfigContent, err := json.Marshal(machineConfig)
	if err != nil {
		return errors.Wrap(err, "populateFields marshall machineConfig")
	}
	if m.Data == nil {
		m.Data = map[string]interface{}{}
	}
	fields, ok := m.Data["fields"].(map[string]interface{})
	if !ok {
		fields = map[string]interface{}{}
	}
	driverConfig, ok := fields[m.Driver+"Config"].(map[string]interface{})
	if !ok {
		driverConfig = map[string]interface{}{}
	}
	if err := json.Unmarshal(machineConfigContent, &driverConfig); err != nil {
		return errors.Wrap(err, "populateFields unmarshall to fields")
	}
	for _, key := range []string{"id", "type", "links", "actions"} {
		delete(driverConfig, key)
	}
	fields[m.Driver+"Config"] = driverConfig
	m.Data["fields"] = fields
	return nil
}

func copyData(m *client.Machine, from interface{}) error {
	content, err := json.Marshal(from)
	if err != nil {
		return errors.Wrap(err, "copyData marshall")
	}
	err = json.Unmarshal(content, m)
	if err != nil {
		return errors.Wrap(err, "copyData unmarshall")
	}
	fields := m.Data["fields"]
	err = json.Unmarshal(content, &fields)
	if err != nil {
		return errors.Wrap(err, "copyData unmarshall to fields")
	}
	m.Data["fields"] = fields
	return nil
}

func getHost(m *client.Machine, apiClient *client.RancherClient) (*client.Host, error) {
	hosts, err := apiClient.Host.List(&client.ListOpts{
		Filters: map[string]interface{}{
			"physicalHostId": m.Id,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(hosts.Data) == 0 {
		return nil, err
	}

	return &hosts.Data[0], nil
}

func notAMachineReply(event *events.Event, apiClient *client.RancherClient) error {
	// Called when machine.ById() returned a 404, which indicates this is a
	// physicalHost but not a machine. Just reply.
	reply := newReply(event)
	return publishReply(reply, apiClient)
}

func newReply(event *events.Event) *client.Publish {
	return &client.Publish{
		Name:        event.ReplyTo,
		PreviousIds: []string{event.ID},
	}
}

func cleanupResources(machineDir, name string) error {
	logger.WithFields(logrus.Fields{
		"machine name": name,
	}).Info("starting cleanup...")
	dExists, err := dirExists(machineDir)
	if !dExists {
		return nil
	}

	mExists, err := machineExists(machineDir, name)
	if err != nil {
		return err
	}

	if !mExists {
		return nil
	}

	command := buildCommand(machineDir, []string{"rm", "-f", name})

	err = command.Start()
	if err != nil {
		return err
	}

	err = command.Wait()
	if err != nil {
		return err
	}

	removeMachineDir(machineDir)

	logger.WithFields(logrus.Fields{
		"machine name": name,
	}).Info("cleanup successful")
	return nil
}

func dirExists(machineDir string) (bool, error) {
	if _, err := os.Stat(machineDir); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

func preEvent(event *events.Event, apiClient *client.RancherClient) (*client.Machine, string, error) {
	machine, err := getMachine(event.ResourceID, apiClient)
	if err != nil {
		return nil, "", err
	}
	if machine == nil {
		return nil, "", notAMachineReply(event, apiClient)
	}

	if machine.Driver == "rancher" {
		addRancherConfig(machine, event)
	}

	machineDir, err := buildBaseMachineDir(machine)
	if err != nil {
		return nil, "", err
	}

	return machine, machineDir, restoreMachineDir(machine, machineDir)
}

func addRancherConfig(machine *client.Machine, event *events.Event) {
	fields := machine.Data["fields"].(map[string]interface{})
	rancherConfig, ok := event.Data["rancherConfig"].(map[string]interface{})
	if ok {
		flavor := rancherConfig["flavor"].(string)
		rancherConfigAppendFieldsData(rancherConfig, fields, flavor[:strings.Index(flavor, "-")])
	} else {
		rancherConfig = map[string]interface{}{}
	}
	fields["rancherConfig"] = rancherConfig
}

func rancherConfigAppendFieldsData(rancherConfig map[string]interface{}, fields map[string]interface{}, provider string) {
	if providerFields := fields[provider+"Config"].(map[string]interface{}); providerFields != nil {
		for k, v := range providerFields {
			rancherConfig[provider+strings.ToUpper(k[:1])+k[1:]] = v
		}
	}
}
