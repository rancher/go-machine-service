package providers

import (
	"github.com/rancher/go-rancher/v3"
)

func init() {
	rackspaceHandler := &RackspaceHandler{}
	if err := RegisterProvider("rackspace", rackspaceHandler); err != nil {
		logger.Fatal("could not register rackspace provider")
	}
}

type RackspaceHandler struct {
}

func (*RackspaceHandler) HandleCreate(host *client.Host, hostDir string) error {
	return nil
}

func (*RackspaceHandler) HandleError(msg string) string {
	prettyMsg := msg
	if msg == "Expected HTTP response code [200 203] when accessing [POST https://identity.api.rackspacecloud.com/v2.0/tokens], but got 401 instead" {
		prettyMsg = "Invalid username or apiKey"
	}
	return prettyMsg
}
