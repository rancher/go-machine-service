package utils

import (
	"bytes"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	MachineCmd   = "machine"
	ParseMessage = "Failed to parse config: [%v]"
)

// Returns the URL at which the Cattle API can be reached.
//
// forRancherAgent indicates if this URL will be used to allow
// a Rancher host agent to communicate with Cattle. Its primary purpose is
// to accomodate local development where cattle is running on localhost but
// Docker is running inside a VM where localhost has a different context.
func GetRancherUrl(forRancherAgent bool) string {
	cattleUrl := os.Getenv("CATTLE_URL")
	if forRancherAgent {
		if override := os.Getenv("CATTLE_URL_FOR_AGENT"); override != "" {
			cattleUrl = override
		}
	}
	return cattleUrl
}

func GetRancherAccessKey() string {
	return os.Getenv("CATTLE_ACCESS_KEY")
}

func GetRancherSecretKey() string {
	return os.Getenv("CATTLE_SECRET")
}

func GetRancherAgentImage() (string, string) {
	return "rancher/agent", "latest"
}

// Returns a TLS-enabled docker client for the specified machine.
func GetDockerClient(machineName string) (*docker.Client, error) {
	conf, err := getConnectionConfig(machineName)
	if err != nil {
		return nil, err
	}

	client, err := docker.NewTLSClient(conf.endpoint, conf.cert, conf.key, conf.caCert)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func FindContainersByNames(client *docker.Client, names ...string) ([]docker.APIContainers, error) {
	listOpts := docker.ListContainersOptions{
		All:     true,
		Filters: map[string][]string{"name": names},
	}
	return client.ListContainers(listOpts)
}

type tlsConnectionConfig struct {
	endpoint string
	cert     string
	key      string
	caCert   string
}

func getConnectionConfig(machineName string) (*tlsConnectionConfig, error) {
	cmd := exec.Command(MachineCmd, "config", machineName)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	args := string(bytes.TrimSpace(output))

	connConfig, err := parseConnectionArgs(args)
	if err != nil {
		return nil, err
	}

	return connConfig, nil
}

func parseConnectionArgs(args string) (*tlsConnectionConfig, error) {
	// Extract the -H (host) parameter
	endpointRegEx := regexp.MustCompile("-H=\".*\"")
	endpointMatches := endpointRegEx.FindAllString(args, -1)
	if len(endpointMatches) != 1 {
		return nil, fmt.Errorf(ParseMessage, args)
	}
	endpointKV := strings.Split(endpointMatches[0], "=")
	if len(endpointKV) != 2 {
		return nil, fmt.Errorf(ParseMessage, args)
	}
	endpoint := strings.Replace(endpointKV[1], "\"", "", -1)
	config := &tlsConnectionConfig{endpoint: endpoint}
	args = endpointRegEx.ReplaceAllString(args, "")

	// Extract the tls args: tlscacert tlscert tlskey
	whitespaceSplit := regexp.MustCompile("\\w*--")
	tlsArgs := whitespaceSplit.Split(args, -1)
	for _, arg := range tlsArgs {
		kv := strings.Split(arg, "=")
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			val := strings.TrimSpace(kv[1])
			switch key {
			case "tlscacert":
				config.caCert = val
			case "tlscert":
				config.cert = val
			case "tlskey":
				config.key = val
			}
		}
	}

	return config, nil
}
