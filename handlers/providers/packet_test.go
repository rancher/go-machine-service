package providers

import (
	"testing"
)

func TestPacketErrorHandler(t *testing.T) {
	packetHandler := &PacketHandler{}

	msg := "POST https://api.packet.net/projects/ 404"
	expectedPrettyMessage := "Invalid project"
	actualPrettyMessage := packetHandler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}

	msg = "GET https://api.packet.net/facilities: 401"
	expectedPrettyMessage = "Invalid API key"
	actualPrettyMessage = packetHandler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}

	msg = "everything else under the sun."
	expectedPrettyMessage = "everything else under the sun."
	actualPrettyMessage = packetHandler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}
}
