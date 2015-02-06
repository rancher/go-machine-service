package utils

import (
	"fmt"
	"testing"
)

// Tests the simplest case of successfully receiving, routing, and handling
// three events.
func TestParseConnectionArgs(t *testing.T) {
	ca := "/foo/bar/ca.pem"
	cert := "/foo/bar\\ baz/cert.pem"
	key := "/foo/bar/key.pem"
	endpoint := "tcp://127.0.0.1:2376"

	testArgs := fmt.Sprintf("--tls --tlscacert=%v   --tlscert=%v --tlskey=%v -H=\"%v\" ",
		ca, cert, key, endpoint)
	testParse(testArgs, ca, cert, key, endpoint, t)

	testArgs = fmt.Sprintf("-H=\"%v\" --tlscacert=%v --tlscert=%v --tlskey=%v --tls",
		endpoint, ca, cert, key)
	testParse(testArgs, ca, cert, key, endpoint, t)

	testArgs = fmt.Sprintf("--tlscacert=%v --tlscert=%v --tlskey=%v --tls", ca, cert, key)
	_, err := parseConnectionArgs(testArgs)
	if err == nil {
		t.Fatalf("Parse should have failed because of missing host.")
	}

}

func testParse(testArgs, ca, cert, key, endpoint string, t *testing.T) {
	config, err := parseConnectionArgs(testArgs)
	if err != nil {
		t.Fatalf("Error parsing.", err)
	}
	checkField("endpoint", endpoint, config.endpoint, t)
	checkField("ca", ca, config.caCert, t)
	checkField("cert", cert, config.cert, t)
	checkField("key", key, config.key, t)
}

func checkField(field, expected, actual string, t *testing.T) {
	if expected != actual {
		t.Logf("Mismatch on field [%v]. Expected: [%v],  Actual: [%v]", field, expected, actual)
		t.FailNow()
	}
}
