package utils

import (
	"bytes"
	"github.com/fsouza/go-dockerclient"
	"os"
	"os/exec"
	"path/filepath"
)

const MachineCmd = "machine"

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

func GetRancherAgentImage() (string, string) {
	return "cjellick/agent", "latest"
}

func getMachineStoragePath() string {
	storagePath := os.Getenv("MACHINE_STORAGE_PATH")
	if storagePath != "" {
		return storagePath
	} else {
		home := os.Getenv("HOME")
		// TODO Worry about windows. Could use machine's drivers/utils.go
		return filepath.Join(home, ".docker", "machines")
	}
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
	storepath := getMachineStoragePath()
	ca := filepath.Join(storepath, machineName, "ca.pem")
	cert := filepath.Join(storepath, machineName, "cert.pem")
	key := filepath.Join(storepath, machineName, "key.pem")

	cmd := exec.Command(MachineCmd, "url", machineName)
	result, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	endpoint := string(bytes.TrimSpace(result))

	connConfig := &tlsConnectionConfig{
		endpoint: endpoint,
		caCert:   ca,
		cert:     cert,
		key:      key,
	}
	return connConfig, nil
}
