package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"

	"github.com/rancher/event-subscriber/events"
	client "github.com/rancher/go-rancher/v3"
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

func buildBaseHostDir(h *client.Host) (string, error) {
	machineDir := filepath.Join(getWorkDir(), "machines", h.Uuid)
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
	defaultWaitTime := time.Second * 5
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

func applyHostTemplate(host *client.Host, apiClient *client.RancherClient) error {
	if host.HostTemplateId != "" {
		ht, err := apiClient.HostTemplate.ById(host.HostTemplateId)
		if err != nil {
			return err
		}
		return apply(host, ht, apiClient, true)
	}

	templates, err := apiClient.HostTemplate.List(&client.ListOpts{
		Filters: map[string]interface{}{
			"clusterId":    host.ClusterId,
			"driver":       host.Driver,
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
			if err := apply(host, &ht, apiClient, false); err != nil {
				return err
			}
		}
	} else if len(templates.Data) == 1 {
		return apply(host, &templates.Data[0], apiClient, true)
	}

	return nil
}

func apply(host *client.Host, ht *client.HostTemplate, apiClient *client.RancherClient, public bool) error {
	if public {
		if err := copyData(host, ht.PublicValues); err != nil {
			return err
		}
	}

	secretValues := map[string]interface{}{}
	if err := apiClient.GetLink(ht.Resource, "secretValues", &secretValues); err != nil {
		return errors.Wrap(err, "Get secretValues link")
	}

	err := copyData(host, secretValues)
	if err != nil {
		return err
	}
	if err := populateFields(host); err != nil {
		return err
	}

	return nil
}

func populateFields(m *client.Host) error {
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

func copyData(host *client.Host, from interface{}) error {
	content, err := json.Marshal(from)
	if err != nil {
		return errors.Wrap(err, "copyData marshall")
	}
	err = json.Unmarshal(content, host)
	if err != nil {
		return errors.Wrap(err, "copyData unmarshall")
	}
	fields := host.Data["fields"]
	err = json.Unmarshal(content, &fields)
	if err != nil {
		return errors.Wrap(err, "copyData unmarshall to fields")
	}
	host.Data["fields"] = fields
	return nil
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

	os.RemoveAll(machineDir)

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

func getHostAndHostDir(event *events.Event, apiClient *client.RancherClient) (*client.Host, string, error) {
	host, err := apiClient.Host.ById(event.ResourceID)
	if err != nil {
		return nil, "", err
	}
	if host == nil {
		return nil, "", errors.Errorf("can't find host with resourceId %v", host.Id)
	}

	hostDir, err := buildBaseHostDir(host)
	if err != nil {
		return nil, "", err
	}

	return host, hostDir, nil
}
