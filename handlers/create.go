package handlers

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	b64 "encoding/base64"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-rancher/client"
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
	levelInfo            = "level=\"info\""
	levelError           = "level=\"error\""
	errorCreatingMachine = "Error creating machine: "
)

var RegExDockerMsg = regexp.MustCompile("msg=.*")

func CreateMachine(event *events.Event, apiClient *client.RancherClient) error {
	log.WithFields(log.Fields{
		"resourceId": event.ResourceId,
		"eventId":    event.Id,
	}).Info("Creating Machine")

	machine, err := getMachine(event.ResourceId, apiClient)
	if err != nil {
		return handleByIdError(err, event, apiClient)
	}

	// Idempotency. If the resource has the property, we're done.
	if _, ok := machine.Data[machineDirField]; ok {
		reply := newReply(event)
		return publishReply(reply, apiClient)
	}

	command, machineDir, err := buildCreateCommand(machine)
	if err != nil {
		return err
	}

	//Setup republishing timer
	publishChan := make(chan string, 10)
	go republishTransitioningReply(publishChan, event, apiClient)

	publishChan <- "Contacting " + machine.Driver
	alreadyClosed := false
	defer func() {
		if !alreadyClosed {
			close(publishChan)
		}
	}()

	readerStdout, readerStderr, err := startReturnOutput(command)
	if err != nil {
		return err
	}

	errChan := make(chan string, 1)
	go logProgress(readerStdout, readerStderr, publishChan, machine, event, errChan)

	err = command.Wait()

	if err != nil {
		select {
		case errString := <-errChan:
			if errString != "" {
				return fmt.Errorf(errString)
			}
		case <-time.After(10 * time.Second):
			log.Error("Waited 10 seconds to break after command.Wait().  Please review logProgress.")
		}
		return err
	}

	dataUpdates := map[string]interface{}{machineDirField: machineDir}
	eventDataWrapper := map[string]interface{}{"+data": dataUpdates}

	log.WithFields(log.Fields{
		"resourceId":        event.ResourceId,
		"machineExternalId": machine.ExternalId,
	}).Info("Machine Created")

	destFile, err := createExtractedConfig(event, machine)
	if err != nil {
		return err
	}

	if destFile != "" {
		publishChan <- "Saving Machine Config"
		extractedConf, err := getExtractedConfig(destFile, machine, apiClient)
		if err != nil {
			return err
		}
		dataUpdates["+fields"] = map[string]string{"extractedConfig": extractedConf}
	}

	reply := newReply(event)
	reply.Data = eventDataWrapper

	// Explicitly close publish channel so that it doesn't conflict with final reply
	close(publishChan)
	alreadyClosed = true
	return publishReply(reply, apiClient)
}

func logProgress(readerStdout io.Reader, readerStderr io.Reader, publishChan chan<- string, machine *client.Machine, event *events.Event, errChan chan<- string) {
	// We will just log stdout first, then stderr, ignoring all errors.
	defer close(errChan)
	scanner := bufio.NewScanner(readerStdout)
	for scanner.Scan() {
		msg := scanner.Text()
		log.WithFields(log.Fields{
			"resourceId: ": event.ResourceId,
		}).Infof("stdout: %s", msg)
		msg = filterDockerMessage(msg, machine, errChan)
		if msg != "" {
			publishChan <- msg
		}
	}
	scanner = bufio.NewScanner(readerStderr)
	for scanner.Scan() {
		log.WithFields(log.Fields{
			"resourceId": event.ResourceId,
		}).Infof("stderr: %s", scanner.Text())
	}
}

func filterDockerMessage(msg string, machine *client.Machine, errChan chan<- string) string {
	// Docker log messages come in the format: time=<t> level=<log-level> msg=<message>
	// The minimum string should be greater than 7 characters msg="."
	match := RegExDockerMsg.FindString(msg)
	if len(match) < 7 || !strings.HasPrefix(match, "msg=\"") {
		return ""
	}

	match = (match[5 : len(strings.TrimSpace(match))-1])
	if strings.Contains(msg, levelInfo) {
		// We just want to return <message> to cattle and only messages that do not contain the machine uuid or namee
		if strings.Contains(msg, machine.ExternalId) || strings.Contains(msg, machine.Name) {
			return ""
		}
		return match
	} else if strings.Contains(msg, levelError) {
		errChan <- strings.Replace(match, errorCreatingMachine, "", 1)
		return ""
	}
	return ""
}

func startReturnOutput(command *exec.Cmd) (io.Reader, io.Reader, error) {
	readerStdout, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	readerStderr, err := command.StderrPipe()
	if err != nil {
		return nil, nil, err
	}

	err = command.Start()
	if err != nil {
		defer readerStdout.Close()
		defer readerStderr.Close()
		return nil, nil, err
	}
	return readerStdout, readerStderr, nil
}

func buildCreateCommand(machine *client.Machine) (*exec.Cmd, string, error) {
	cmdArgs, err := buildMachineCreateCmd(machine)
	if err != nil {
		return nil, "", err
	}

	machineDir, err := buildBaseMachineDir(machine.ExternalId)
	if err != nil {
		return nil, "", err
	}

	command := buildCommand(machineDir, cmdArgs)
	return command, machineDir, nil
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
		return "", fmt.Errorf("CATTLE_HOME not set. Cant create machine. Uuid: [%v].", uuid)
	}
	machineDir := filepath.Join(cattleHome, "machine", uuid)
	return machineDir, nil
}

func buildMachineCreateCmd(machine *client.Machine) ([]string, error) {
	// TODO Quick and dirty. Refactor to use reflection and maps
	// TODO Write a separate test for this function
	cmd := []string{"create", "-d"}

	switch strings.ToLower(machine.Driver) {
	case "digitalocean":
		cmd = append(cmd, "digitalocean")
		if machine.DigitaloceanConfig.Image != "" {
			cmd = append(cmd, "--digitalocean-image", machine.DigitaloceanConfig.Image)
		}
		if machine.DigitaloceanConfig.Size != "" {
			cmd = append(cmd, "--digitalocean-size", machine.DigitaloceanConfig.Size)
		}
		if machine.DigitaloceanConfig.Region != "" {
			cmd = append(cmd, "--digitalocean-region", machine.DigitaloceanConfig.Region)
		}
		if machine.DigitaloceanConfig.AccessToken != "" {
			cmd = append(cmd, "--digitalocean-access-token", machine.DigitaloceanConfig.AccessToken)
		}
		if machine.DigitaloceanConfig.Ipv6 {
			cmd = append(cmd, "--digitalocean-ipv6")
		}
		if machine.DigitaloceanConfig.PrivateNetworking {
			cmd = append(cmd, "--digitalocean-private-networking")
		}
		if machine.DigitaloceanConfig.Backups {
			cmd = append(cmd, "--digitalocean-backups")
		}
	case "amazonec2":
		cmd = append(cmd, "amazonec2")
		if machine.Amazonec2Config.AccessKey != "" {
			cmd = append(cmd, "--amazonec2-access-key", machine.Amazonec2Config.AccessKey)
		}
		if machine.Amazonec2Config.SecretKey != "" {
			cmd = append(cmd, "--amazonec2-secret-key", machine.Amazonec2Config.SecretKey)
		}
		if machine.Amazonec2Config.VpcId != "" {
			cmd = append(cmd, "--amazonec2-vpc-id", machine.Amazonec2Config.VpcId)
		}
		if machine.Amazonec2Config.Ami != "" {
			cmd = append(cmd, "--amazonec2-ami", machine.Amazonec2Config.Ami)
		}
		if machine.Amazonec2Config.SessionToken != "" {
			cmd = append(cmd, "--amazonec2-session-token", machine.Amazonec2Config.SessionToken)
		}
		if machine.Amazonec2Config.Region != "" {
			cmd = append(cmd, "--amazonec2-region", machine.Amazonec2Config.Region)
		}
		if machine.Amazonec2Config.Zone != "" {
			cmd = append(cmd, "--amazonec2-zone", machine.Amazonec2Config.Zone)
		}
		if machine.Amazonec2Config.SubnetId != "" {
			cmd = append(cmd, "--amazonec2-subnet-id", machine.Amazonec2Config.SubnetId)
		}
		if machine.Amazonec2Config.SecurityGroup != "" {
			cmd = append(cmd, "--amazonec2-security-group", machine.Amazonec2Config.SecurityGroup)
		}
		if machine.Amazonec2Config.InstanceType != "" {
			cmd = append(cmd, "--amazonec2-instance-type", machine.Amazonec2Config.InstanceType)
		}
		if machine.Amazonec2Config.RootSize != "" {
			cmd = append(cmd, "--amazonec2-root-size", machine.Amazonec2Config.RootSize)
		}
		if machine.Amazonec2Config.IamInstanceProfile != "" {
			cmd = append(cmd, "--amazonec2-iam-instance-profile", machine.Amazonec2Config.AccessKey)
		}
	case "virtualbox":
		cmd = append(cmd, "virtualbox")
		if machine.VirtualboxConfig.Boot2dockerUrl != "" {
			cmd = append(cmd, "--virtualbox-boot2docker-url", machine.VirtualboxConfig.Boot2dockerUrl)
		}
		if machine.VirtualboxConfig.DiskSize != "" {
			cmd = append(cmd, "--virtualbox-disk-size", machine.VirtualboxConfig.DiskSize)
		}
		if machine.VirtualboxConfig.Memory != "" {
			cmd = append(cmd, "--virtualbox-memory", machine.VirtualboxConfig.Memory)
		}
	default:
		return nil, fmt.Errorf("Unrecognize Driver: %v", machine.Driver)
	}

	cmd = append(cmd, machine.Name)

	log.Infof("Cmd slice: %v", cmd)
	return cmd, nil
}

func createExtractedConfig(event *events.Event, machine *client.Machine) (string, error) {
	// We are going to ignore doing anything for VirtualBox given that there is no way you can
	// use Machine once it has been created.  Virtual is mainly a test-only use case
	if strings.ToLower(machine.Driver) == "virtualbox" {
		log.Debug("VirtualBox machine does not need config extracted")
		return "", nil
	}

	// We will now zip, base64 encode the machine directory created, and upload this to cattle server.  This can be used to either recover
	// the machine directory or used by Rancher users for their own local machine setup.
	log.WithFields(log.Fields{
		"resourceId": event.ResourceId,
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

		// For now, we will skip directories.  If we need to zip directories, we need to revist this code
		if fileInfo.IsDir() {
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
