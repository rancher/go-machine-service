package providers

import (
	"testing"
)

func TestDigitalOceanErrorHandler(t *testing.T) {
	digitaloceanHandler := &DigitaloceanHandler{}

	msg := "401 Unable to authenticate you."
	expectedPrettyMessage := "Invalid access token"
	actualPrettyMessage := digitaloceanHandler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}

	msg = "everything else under the sun."
	expectedPrettyMessage = "everything else under the sun."
	actualPrettyMessage = digitaloceanHandler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}
}
