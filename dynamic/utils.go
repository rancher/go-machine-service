package dynamic

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rancher/go-rancher/v2"
)

var (
	maxWait = 1 * time.Second
)

func getClient() (*client.RancherClient, error) {
	apiURL := os.Getenv("CATTLE_URL")
	accessKey := os.Getenv("CATTLE_ACCESS_KEY")
	secretKey := os.Getenv("CATTLE_SECRET_KEY")

	return client.NewRancherClient(&client.ClientOpts{
		Url:       apiURL,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Timeout:   time.Second * 60,
	})
}

func waitSchema(schema client.DynamicSchema, apiClient *client.RancherClient) error {
	// 2-ish minute timeout
	wait := 100 * time.Millisecond
	for i := 0; i < 120; i++ {
		gotSchema, err := apiClient.DynamicSchema.ById(schema.Id)
		if err != nil {
			return err
		}
		if gotSchema.Transitioning != "yes" {
			return nil
		}
		time.Sleep(wait)
		wait = wait * 2
		if wait > maxWait {
			wait = maxWait
		}
	}
	return fmt.Errorf("Timeout waiting for schema %s %s", schema.Id, schema.Name)
}

func toJSON(obj interface{}) (string, error) {
	fieldsJSON, err := json.MarshalIndent(obj, "", "    ")
	return string(fieldsJSON), err
}

func toLowerCamelCase(driver, machineFlagName string) (string, error) {
	var parts []string
	if strings.HasPrefix(machineFlagName, driver) {
		parts = strings.SplitN(machineFlagName, "-", 2)
		if len(parts) > 1 {
			parts = strings.Split(parts[1], "-")
		}
	} else {
		parts = strings.Split(machineFlagName, "-")
	}

	flagName := parts[0]
	for _, part := range parts[1:] {
		flagName = flagName + strings.ToUpper(part[:1]) + part[1:]
	}
	return flagName, nil
}
