package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestDigitalOcean(t *testing.T) {
	accessToken := os.Getenv("DIGITALOCEAN_KEY")
	if accessToken == "" {
		t.Log("Skipping Digital Ocean test.")
		return
	}
	setupDO(accessToken)

	resourceID := "gmstest-" + strconv.FormatInt(time.Now().Unix(), 10)
	event := &events.Event{
		ResourceID: resourceID,
		ID:         "event-id",
		ReplyTo:    "reply-to-id",
	}
	mockAPIClient := &client.RancherClient{}

	err := CreateMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	err = ActivateMachine(event, mockAPIClient)
	if err != nil {
		// Fail, not a fatal, so purge will still run.
		t.Log(err)
		t.Fail()
	}

	err = PurgeMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}
}

func TestHADigitalOcean(t *testing.T) {
	accessToken := os.Getenv("DIGITALOCEAN_KEY")
	if accessToken == "" {
		t.Log("Skipping Digital Ocean test.")
		return
	}
	setupDO(accessToken)

	resourceID := "gmstest-" + strconv.FormatInt(time.Now().Unix(), 10)
	event := &events.Event{
		ResourceID: resourceID,
		ID:         "event-id",
		ReplyTo:    "reply-to-id",
	}
	mockAPIClient := &client.RancherClient{}

	err := CreateMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	machine, err := getMachine(resourceID, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	machineDir, err := getMachineDir(machine)
	if err != nil {
		t.Fatal(err)
	}

	dropletID, err := getDropletID(machineDir, machine.Name)
	if err != nil {
		t.Fatal(err)
	}

	err = waitForDropletStatusCode(200, dropletID, accessToken)
	if err != nil {
		t.Fatal(err)
	}

	machine, err = getMachine(resourceID, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	err = os.RemoveAll(machineDir)
	if err != nil {
		t.Fatal(err)
	}

	err = ActivateMachine(event, mockAPIClient)
	if err != nil {
		// Fail, not a fatal, so purge will still run.
		t.Fatal(err)
	}

	err = os.RemoveAll(machineDir)
	if err != nil {
		t.Fatal(err)
	}

	err = PurgeMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	// Delete it twice just to prove that works
	err = PurgeMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	// Unset extractedConfig to prove it doesn't error out
	machine.ExtractedConfig = ""
	err = PurgeMachine(event, mockAPIClient)
	if err != nil {
		t.Fatal(err)
	}

	err = waitForDropletStatusCode(404, dropletID, accessToken)
	if err != nil {
		t.Fatal(err)
	}
}

func waitForDropletStatusCode(expectedStatus int, dropletID int, accessToken string) error {
	statusCode := -1
	for i := 0; i < 30; i++ {
		client := http.DefaultClient
		url := fmt.Sprintf("https://api.digitalocean.com/v2/droplets/%v", dropletID)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(accessToken, "")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		statusCode = resp.StatusCode
		if statusCode == expectedStatus {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("Timed out waiting for status [%v] on droplet [%v]. Last status: [%v].", expectedStatus, dropletID, statusCode)
}

func getDropletID(machineDir, name string) (int, error) {
	configFile := filepath.Join(machineDir, "machines", name, "config.json")

	file, err := ioutil.ReadFile(configFile)
	if err != nil {
		return 0, err
	}

	config := &configJson{}
	//fmt.Printf("yo: %s\n", string(file))
	err = json.Unmarshal(file, config)
	if err != nil {
		return 0, err
	}

	return config.Driver.DropletID, nil
}

type driverJson struct {
	DropletID int `json:"DropletID"`
}

type configJson struct {
	Driver driverJson `json:"Driver"`
}

func setupDO(accessToken string) {
	// TODO Replace functions during teardown.
	data := make(map[string]interface{})
	fields := make(map[string]interface{})
	data["fields"] = fields
	digitaloceanConfig := make(map[string]interface{})
	fields["digitaloceanConfig"] = digitaloceanConfig

	digitaloceanConfig["accessToken"] = accessToken
	digitaloceanConfig["region"] = "sfo1"
	digitaloceanConfig["size"] = "1gb"
	digitaloceanConfig["image"] = "ubuntu-14-04-x64"
	digitaloceanConfig["ipv6"] = true
	digitaloceanConfig["backups"] = false
	digitaloceanConfig["privateNetworking"] = true

	machine := &client.Machine{
		Data:             data,
		EngineInstallUrl: "https://test.docker.com/",
		Kind:             "machine",
		Driver:           "digitalocean",
	}

	getMachine = func(id string, apiClient *client.RancherClient) (*client.Machine, error) {
		machine.Id = id
		machine.Name = "name-" + id
		machine.ExternalId = "ext-" + id
		return machine, nil
	}

	getRegistrationURLAndImage = func(accountId string, apiClient *client.RancherClient) (string, string, string, error) {
		return "http://1.2.3.4/v1", "rancher/agent", "v0.7.6", nil
	}

	publishReply = buildMockPublishReply(machine)
	publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {}
}
