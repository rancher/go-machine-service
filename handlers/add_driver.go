package handlers

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/go-machine-service/events"
	"github.com/rancher/go-rancher/client"
)

const (
	driverBinaryPrefix = "docker-machine-driver-"
)

type teeWriter struct {
	dst1 io.Writer
	dst2 io.Writer
}

func (tw *teeWriter) Write(p []byte) (n int, err error) {
	n, err = tw.dst1.Write(p)
	if err != nil {
		return n, err
	}
	return tw.dst2.Write(p)
}

func AddDriver(event *events.Event, client *client.RancherClient) error {
	log.WithFields(log.Fields{
		"resourceId": event.ResourceId,
		"eventId":    event.Id,
	}).Info("Adding Machine Driver")

	downloadUrl := "" //obtain driver download url
	checksum := ""    //obtain checksum
	driverName := ""  //obtain driver name

	//download to temp file first in case something goes wrong during the download
	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer tempFile.Close()
	md5Hash := md5.New()
	tw := &teeWriter{tempFile, md5Hash}

	err = download_driver(downloadUrl, tw)
	if err != nil {
		return err
	}

	//NOTE: Adding a implicit constraint that checksum should be a md5 hash. The value should be the lowercase string represention of the hash as a hexadecimal value
	if fmt.Sprintf("%x", md5Hash.Sum(nil)) != checksum {
		return err
	}

	return os.Rename(tempFile.Name(), filepath.Join("/usr/local/bin", fmt.Sprintf("%s%s", driverBinaryPrefix, driverName)))
}

func download_driver(url string, out io.Writer) error {
	//Default Timeout and KeepAlives are good
	client := &http.Client{}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
