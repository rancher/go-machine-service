package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/event-subscriber/events"
	"github.com/rancher/go-rancher/v2"
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

var errNoExtractedConfig = errors.New("Machine does not have an saved config. Cannot reinitialize")

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

func reinitFromExtractedConfig(machine *client.Machine, machineBaseDir string) error {
	if err := os.MkdirAll(machineBaseDir, 0740); err != nil {
		return fmt.Errorf("Error reinitializing config (MkdirAll). Config Dir: %v. Error: %v", machineBaseDir, err)
	}

	if machine.ExtractedConfig == "" {
		return errNoExtractedConfig
	}

	configBytes, err := b64.StdEncoding.DecodeString(machine.ExtractedConfig)
	if err != nil {
		return fmt.Errorf("Error reinitializing config (base64.DecodeString). Config Dir: %v. Error: %v", machineBaseDir, err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(configBytes))
	if err != nil {
		return err
	}
	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("Error reinitializing config (tarRead.Next). Config Dir: %v. Error: %v", machineBaseDir, err)
		}

		filename := header.Name
		filePath := filepath.Join(machineBaseDir, filename)
		log.Infof("Extracting %v", filePath)

		info := header.FileInfo()
		if info.IsDir() {
			err = os.MkdirAll(filePath, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("Error reinitializing config (Mkdirall). Config Dir: %v. Dir: %v. Error: %v", machineBaseDir, info.Name(), err)
			}
			continue
		}

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return fmt.Errorf("Error reinitializing config (OpenFile). Config Dir: %v. File: %v. Error: %v", machineBaseDir, info.Name(), err)
		}
		defer file.Close()
		_, err = io.Copy(file, tarReader)
		if err != nil {
			return fmt.Errorf("Error reinitializing config (Copy). Config Dir: %v. File: %v. Error: %v", machineBaseDir, info.Name(), err)
		}
	}
}

func createExtractedConfig(event *events.Event, machine *client.Machine) (string, error) {
	// We are going to ignore doing anything for VirtualBox given that there is no way you can
	// use Machine once it has been created.  Virtual is mainly a test-only use case
	if ignoreExtractedConfig(machine.Driver) {
		log.Debug("VirtualBox machine does not need config extracted")
		return "", nil
	}

	log.WithFields(log.Fields{
		"resourceId": event.ResourceID,
	}).Info("Creating and uploading extracted machine config")

	baseDir, err := getBaseMachineDir(machine.ExternalId)
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

	if err := addDirToArchive(baseDir, tarfileWriter); err != nil {
		return "", err
	}

	return destFile, nil
}

func addDirToArchive(source string, tarfileWriter *tar.Writer) error {
	baseDir := filepath.Base(source)

	return filepath.Walk(source,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if path == source || strings.HasSuffix(info.Name(), ".iso") ||
				strings.HasSuffix(info.Name(), ".tar.gz") ||
				strings.HasSuffix(info.Name(), ".img") {
				return nil
			}

			header, err := tar.FileInfoHeader(info, info.Name())
			if err != nil {
				return err
			}

			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))

			if err := tarfileWriter.WriteHeader(header); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()
			_, err = io.Copy(tarfileWriter, file)
			return err
		})
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

func dirExists(machineDir string) (bool, error) {
	_, err := os.Stat(machineDir)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
