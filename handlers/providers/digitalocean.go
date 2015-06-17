package providers

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-rancher/client"
)

func init() {
	digitaloceanHandler := &DigitaloceanHandler{}
	if err := RegisterProvider("digitalocean", digitaloceanHandler); err != nil {
		log.Fatal("could not register digitalocean provider")
	}
}

type DigitaloceanHandler struct {
}

func (*DigitaloceanHandler) HandleCreate(machine *client.Machine, machineDir string) error {
	return nil
}

func (*DigitaloceanHandler) HandleError(msg string) string {
	prettyMsg := msg
	if strings.Contains(msg, "401 Unable to authenticate you.") {
		prettyMsg = "Invalid access token"
	}
	return prettyMsg
}
