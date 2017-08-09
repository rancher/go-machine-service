package providers

import (
	"strings"

	"github.com/rancher/go-rancher/v3"
)

func init() {
	packetHandler := &PacketHandler{}
	if err := RegisterProvider("packet", packetHandler); err != nil {
		logger.Fatal("could not register packet provider")
	}
}

type PacketHandler struct {
}

func (*PacketHandler) HandleCreate(host *client.Host, hostDir string) error {
	return nil
}

func (*PacketHandler) HandleError(msg string) string {
	prettyMsg := msg
	if strings.Contains(prettyMsg, "POST https://api.packet.net/projects/") && strings.Contains(prettyMsg, "404") {
		prettyMsg = "Invalid project"
	}
	if strings.Contains(prettyMsg, "GET https://api.packet.net/facilities: 401") {
		prettyMsg = "Invalid API key"
	}
	return prettyMsg
}
