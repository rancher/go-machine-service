package dynamicDrivers

import (
	"github.com/rancher/go-rancher/client"
	"os"
	"time"
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
