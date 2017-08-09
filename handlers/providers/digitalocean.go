package providers

import (
	"strings"

	"github.com/rancher/go-rancher/v3"
)

func init() {
	digitaloceanHandler := &DigitaloceanHandler{}
	if err := RegisterProvider("digitalocean", digitaloceanHandler); err != nil {
		logger.Fatal("could not register digitalocean provider")
	}
}

type DigitaloceanHandler struct {
}

func (*DigitaloceanHandler) HandleCreate(host *client.Host, hostDir string) error {
	return nil
}

func (*DigitaloceanHandler) HandleError(msg string) string {
	prettyMsg := msg
	if strings.Contains(msg, "401 Unable to authenticate you.") {
		prettyMsg = "Invalid access token"
	}
	return prettyMsg
}
