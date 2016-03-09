package handlers

import (
	"archive/tar"
	"compress/gzip"
	b64 "encoding/base64"
	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	machineDirEnvKey  = "MACHINE_STORAGE_PATH="
	machineDirField   = "machineDir"
	machineCmd        = "docker-machine"
	defaultCattleHome = "/var/lib/cattle"
)

var RegExMachineDirEnv = regexp.MustCompile("^" + machineDirEnvKey + ".*")

var RegExMachinePluginToken = regexp.MustCompile("^" + "MACHINE_PLUGIN_TOKEN=" + ".*")

var RegExMachineDriverName = regexp.MustCompile("^" + "MACHINE_PLUGIN_DRIVER_NAME=" + ".*")

func PingNoOp(event *events.Event, apiClient *client.RancherClient) error {
	// No-op ping handler
	return nil
}

func getMachineDir(machine *client.Machine) (string, error) {
	return getBaseMachineDir(machine.ExternalId)
}

func buildBaseMachineDir(uuid string) (string, error) {
	machineDir, err := getBaseMachineDir(uuid)
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(machineDir, 0740)
	if err != nil {
		return "", err
	}
	return machineDir, err
}

func getBaseMachineDir(uuid string) (string, error) {
	cattleHome := os.Getenv("CATTLE_HOME")
	if cattleHome == "" {
		cattleHome = defaultCattleHome
	}
	machineDir := filepath.Join(cattleHome, "machine", uuid)
	return machineDir, nil
}

var publishReply = func(reply *client.Publish, apiClient *client.RancherClient) error {
	_, err := apiClient.Publish.Create(reply)
	return err
}

var publishTransitioningReply = func(msg string, event *events.Event, apiClient *client.RancherClient) {
	// Since this is only updating the msg for the state transition, we will ignore errors here
	replyT := newReply(event)
	replyT.Transitioning = "yes"
	replyT.TransitioningMessage = msg
	publishReply(replyT, apiClient)
}

func republishTransitioningReply(publishChan <-chan string, event *events.Event, apiClient *client.RancherClient) {
	// We only do this because there is a current issue within Cattle that if a transition message
	// has not been updated for a period of time, it can no longer be updated.  For now, to deal with this
	// we will simply republish transitioning messages until the next one is added.
	// Because this ticker is going to republish every X seconds, it's will most likely republish a message sooner
	// In all likelihood, we will remove this method later.
	defaultWaitTime := time.Second * 15
	ticker := time.NewTicker(defaultWaitTime)
	var lastMsg string
	for {
		select {
		case msg, more := <-publishChan:
			if !more {
				ticker.Stop()
				return
			}
			lastMsg = msg
			publishTransitioningReply(lastMsg, event, apiClient)

		case <-ticker.C:
			//republish last message
			if lastMsg != "" {
				publishTransitioningReply(lastMsg, event, apiClient)
			}
		}
	}
}

var getMachine = func(id string, apiClient *client.RancherClient) (*client.Machine, error) {
	return apiClient.Machine.ById(id)
}

func notAMachineReply(event *events.Event, apiClient *client.RancherClient) error {
	// Called when machine.ById() returned a 404, which indicates this is a
	// physicalHost but not a machine. Just reply.
	reply := newReply(event)
	return publishReply(reply, apiClient)
}

func newReply(event *events.Event) *client.Publish {
	return &client.Publish{
		Name:        event.ReplyTo,
		PreviousIds: []string{event.ID},
	}
}

func cleanupResources(machineDir, name string) error {
	log.WithFields(log.Fields{
		"machine name": name,
	}).Info("starting cleanup...")
	dExists, err := dirExists(machineDir)
	if !dExists {
		return nil
	}

	mExists, err := machineExists(machineDir, name)
	if err != nil {
		return err
	}

	if !mExists {
		return nil
	}

	command := buildCommand(machineDir, []string{"rm", "-f", name})

	err = command.Start()
	if err != nil {
		return err
	}

	err = command.Wait()
	if err != nil {
		return err
	}

	err = os.RemoveAll(machineDir)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"machine name": name,
	}).Info("cleanup successful")
	return nil
}

func buildCommand(machineDir string, cmdArgs []string) *exec.Cmd {
	command := exec.Command(machineCmd, cmdArgs...)
	env := initEnviron(machineDir)
	command.Env = env
	return command
}

func initEnviron(machineDir string) []string {
	env := os.Environ()
	found := false
	for idx, ev := range env {
		if RegExMachineDirEnv.MatchString(ev) {
			env[idx] = machineDirEnvKey + machineDir
			found = true
		}
		if RegExMachinePluginToken.MatchString(ev) {
			env[idx] = ""
		}
		if RegExMachineDriverName.MatchString(ev) {
			env[idx] = ""
		}
	}
	if !found {
		env = append(env, machineDirEnvKey+machineDir)
	}
	return env
}

func createExtractedConfig(event *events.Event, machine *client.Machine) (string, error) {
	// We are going to ignore doing anything for VirtualBox given that there is no way you can
	// use Machine once it has been created.  Virtual is mainly a test-only use case
	if ignoreExtractedConfig(machine.Driver) {
		log.Debug("VirtualBox machine does not need config extracted")
		return "", nil
	}

	// We will now zip, base64 encode the machine directory created, and upload this to cattle server.  This can be used to either recover
	// the machine directory or used by Rancher users for their own local machine setup.
	log.WithFields(log.Fields{
		"resourceId": event.ResourceID,
	}).Info("Creating and uploading extracted machine config")

	// tar.gz the $CATTLE_HOME/machine/<machine-id>/machines/<machine-name> dir (v0.2.0)
	// Open the source directory and read all the files we want to tar.gz
	baseDir, err := getBaseMachineDir(machine.ExternalId)
	if err != nil {
		return "", err
	}

	machineDir := filepath.Join(baseDir, "machines", machine.Name)
	dir, err := os.Open(machineDir)
	if err != nil {
		return "", err
	}
	defer dir.Close()

	log.WithFields(log.Fields{
		"machineDir": machineDir,
	}).Debug("Preparing directory to be tar.gz")

	// Be able to read the files under this dir
	files, err := dir.Readdir(0)
	if err != nil {
		return "", err
	}

	// create the tar.gz file
	destFile := filepath.Join(baseDir, machine.Name+".tar.gz")
	tarfile, err := os.Create(destFile)
	if err != nil {
		return "", err
	}

	defer tarfile.Close()
	fileWriter := gzip.NewWriter(tarfile)
	defer fileWriter.Close()

	tarfileWriter := tar.NewWriter(fileWriter)
	defer tarfileWriter.Close()

	// Read and add files into <machine-name>.tar.gz
	for _, fileInfo := range files {

		// For now, we will skip directories.  If we need to zip directories, we need to revisit this code
		if fileInfo.IsDir() {
			continue
		}

		if strings.HasSuffix(fileInfo.Name(), ".iso") {
			// Ignore b2d ISO
			continue
		}

		file, err := os.Open(filepath.Join(machineDir, fileInfo.Name()))
		if err != nil {
			return "", err
		}

		defer file.Close()

		// prepare the tar header
		header := new(tar.Header)
		header.Name = filepath.Join(machine.Name, fileInfo.Name())
		header.Size = fileInfo.Size()
		header.Mode = int64(fileInfo.Mode())
		header.ModTime = fileInfo.ModTime()

		err = tarfileWriter.WriteHeader(header)
		if err != nil {
			return "", err
		}

		_, err = io.Copy(tarfileWriter, file)
		if err != nil {
			return "", err
		}
	}
	return destFile, nil
}

func ignoreExtractedConfig(driver string) bool {
	return strings.ToLower(driver) == "virtualbox"
}

func getIdempotentExtractedConfig(machine *client.Machine, machineDir string, apiClient *client.RancherClient) (string, error) {
	if ignoreExtractedConfig(machine.Driver) {
		return "", nil
	}
	tarFile := filepath.Join(machineDir, machine.Name+".tar.gz")
	extractedConfig, err := getExtractedConfig(tarFile, machine, apiClient)
	if err != nil {
		return "", err
	}
	return extractedConfig, nil
}

func getExtractedConfig(destFile string, machine *client.Machine, apiClient *client.RancherClient) (string, error) {
	extractedTarfile, err := ioutil.ReadFile(destFile)
	if err != nil {
		return "", err
	}

	extractedEncodedConfig := b64.StdEncoding.EncodeToString(extractedTarfile)
	if err != nil {
		return "", err
	}

	log.WithFields(log.Fields{
		"resourceId": machine.Id,
		"destFile":   destFile,
	}).Info("Machine config file created and encoded.")

	return extractedEncodedConfig, nil
}
