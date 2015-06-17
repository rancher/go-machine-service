package providers

import (
	"testing"
)

func TestAmazonec2ErrorHandler(t *testing.T) {
	amazonec2Handler := &AmazonEC2Handler{}

	msg := "blah blah message=\"Invalid id: ami-15434343\""
	expectedPrettyMessage := "\"Invalid id: ami-15434343\""
	actualPrettyMessage := amazonec2Handler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}

	msg = "everything else under the sun."
	expectedPrettyMessage = "everything else under the sun."
	actualPrettyMessage = amazonec2Handler.HandleError(msg)
	if expectedPrettyMessage != actualPrettyMessage {
		t.Errorf("expected %s, but got %s", expectedPrettyMessage, actualPrettyMessage)
	}
}
