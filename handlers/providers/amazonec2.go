package providers

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-rancher/client"
)

func init() {
	amazonec2Handler := &AmazonEC2Handler{}
	if err := RegisterProvider("amazonec2", amazonec2Handler); err != nil {
		log.Fatal("could not register amazonec2 provider")
	}
}

type AmazonEC2Handler struct {
}

func (*AmazonEC2Handler) HandleCreate(machine *client.Machine, machineDir string) error {
	return nil
}

func (*AmazonEC2Handler) HandleError(msg string) string {
	prettyMsg := msg
	if strings.Contains(msg, "message=") {
		prettyMsg = msg[strings.Index(msg, "message="):]
		prettyMsg = prettyMsg[len("message="):]
	}
	return prettyMsg
}
