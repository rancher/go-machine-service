package providers

import (
	"testing"
)

func TestRackspaceErrorHandler(t *testing.T) {
	rackspaceHandler := &RackspaceHandler{}

	msg := "Expected HTTP response code [200 203] when accessing [POST https://identity.api.rackspacecloud.com/v2.0/tokens], but got 401 instead"
	expectedPrettyMessage := "Invalid username or apiKey"
	actualPrettyMessage := rackspaceHandler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}

	msg = "everything else under the sun."
	expectedPrettyMessage = "everything else under the sun."
	actualPrettyMessage = rackspaceHandler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}
}
