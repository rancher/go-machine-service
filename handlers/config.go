package handlers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	b64 "encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/go-machine-service/logging"
	client "github.com/rancher/go-rancher/v2"
)

var logger = logging.Logger()

func restoreMachineDir(machine *client.Machine, baseDir string) error {
	machineBaseDir := filepath.Dir(baseDir)
	if err := os.MkdirAll(machineBaseDir, 0740); err != nil {
		return fmt.Errorf("Error reinitializing config (MkdirAll). Config Dir: %v. Error: %v", machineBaseDir, err)
	}

	if machine.ExtractedConfig == "" {
		return nil
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
		logger.Infof("Extracting %v", filePath)

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

func createExtractedConfig(baseDir string, machine *client.Machine) (string, error) {
	logger.WithFields(logrus.Fields{
		"resourceId": machine.Id,
	}).Info("Creating and uploading extracted machine config")

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
				strings.HasSuffix(info.Name(), ".vmdk") ||
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

func encodeFile(destFile string) (string, error) {
	extractedTarfile, err := ioutil.ReadFile(destFile)
	if err != nil {
		return "", err
	}

	extractedEncodedConfig := b64.StdEncoding.EncodeToString(extractedTarfile)
	if err != nil {
		return "", err
	}

	return extractedEncodedConfig, nil
}

func saveMachineConfig(machineDir string, machine *client.Machine, apiClient *client.RancherClient) error {
	var err error
	destFile, err := createExtractedConfig(machineDir, machine)
	if err != nil {
		return err
	}

	extractedConf, err := encodeFile(destFile)
	if err != nil {
		return err
	}

	for i := 0; i < 3; i++ {
		_, err = apiClient.Machine.Update(machine, &client.Machine{
			ExtractedConfig: extractedConf,
		})
		if err == nil {
			return err
		}
	}
	return err
}

func removeMachineDir(machineDir string) {
	os.RemoveAll(machineDir)
}
