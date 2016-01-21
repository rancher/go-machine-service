package helpers

import (
	"github.com/rancher/go-rancher/client"
	"os"
)

func getClient() (*client.RancherClient, error) {
	apiUrl := os.Getenv("CATTLE_URL")
	accessKey := os.Getenv("CATTLE_ACCESS_KEY")
	secretKey := os.Getenv("CATTLE_SECRET_KEY")

	return client.NewRancherClient(&client.ClientOpts{

		Url:       apiUrl,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})
}
