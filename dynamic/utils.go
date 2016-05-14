package dynamic

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rancher/go-rancher/client"
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

func toLowerCamelCase(machineFlagName string) (string, error) {
	parts := strings.SplitN(machineFlagName, "-", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("parameter %s does not follow expected naming convention [DRIVER]-[FLAG-NAME]", machineFlagName)
	}
	flagNameParts := strings.Split(parts[1], "-")
	flagName := flagNameParts[0]
	for _, flagNamePart := range flagNameParts[1:] {
		flagName = flagName + strings.ToUpper(flagNamePart[:1]) + flagNamePart[1:]
	}
	return flagName, nil
}
