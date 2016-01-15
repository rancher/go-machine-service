package helpers

import (
	"os"
	"crypto/md5"
	"io"
	"github.com/rancher/go-rancher/client"
	"fmt"
	"net/http"
	"encoding/hex"
	log "github.com/Sirupsen/logrus"
	"os/exec"
	"strings"
	"errors"
	"io/ioutil"
	"hash/fnv"
	"time"
	"math"
	"reflect"
	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	"sync"
)

func computeMd5(filePath string) (string, error) {
	var result []byte
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(result)), nil
}

func tmp_folder(uri string) (string) {
	h := fnv.New32a()
	h.Write([]byte(uri))
	var folder_name =  fmt.Sprintf("%v%v%v","/tmp/", h.Sum32(), "/")
	if _, err := os.Stat(folder_name); os.IsNotExist(err) {
		os.MkdirAll(folder_name, 0777)
	}
	return folder_name
}

func UpdateDrivers() ([]error, error) {
	log.Info("Updating docker-machine-drivers from cattle.")
	if _, err := os.Stat(DRIVERS_LOCATION); os.IsNotExist(err) {
		err2 := os.MkdirAll(DRIVERS_LOCATION, 0777)
		if  err2 != nil  {
			return nil, err2
		}
		log.Debug("Made folder: ",DRIVERS_LOCATION)
	}

	_, errs, err := DownloadDrivers()
	if  err != nil || len(errs) > 0 {
		return errs, err
	}

	time.Sleep(5 * time.Second)
	apiClient, noClient := getClient()

	if noClient != nil {
		return errs, err
	}

	driversCollection, err := apiClient.MachineDriver.List(client.NewListOpts())
	if err != nil {
		return  errs, err
	}

	go killOnChange(driversCollection.Data)
	//TODO put the driver exes on the path
	//TODO need to turn file paths for drivers into correct names.
	//ToDO Generate Schemas And Upload Them To Cattle.
	//	for i,driver := range drivers {
	//		drivers[i] = strings.Replace(path.Base(driver), driverBinaryPrefix,"", 1)
	//	}
	return []error {}, err
}

func killOnChange(startingDrivers []client.MachineDriver) {
	apiClient, failedClientCreation := getClient()
	RECONNECT_ATTEMPTS := 0
	for {
		if failedClientCreation == nil {
			RECONNECT_ATTEMPTS = 0
			drivers, err := apiClient.MachineDriver.List(client.NewListOpts())
			if err == nil {
				if !reflect.DeepEqual(drivers.Data, startingDrivers)  {
					log.Info("Detected change to rancher defined machine drivers. Exiting go machine service.")
					os.Exit(0)
				}
			} else {
				apiClient, failedClientCreation = getClient()
			}
			time.Sleep(time.Second * 5)
		} else {
			apiClient, failedClientCreation = getClient()
			time.Sleep(getTime(RECONNECT_ATTEMPTS))
		}
	}
}

func getTime (reconnectAttempts int) time.Duration {
	totalTime := math.Pow(2, float64(reconnectAttempts))

	if totalTime <= 60 {
		return time.Duration(totalTime) * time.Second
	} else {
		return 60 * time.Second
	}
}

func pickDownloadFileName(driverUri string) string {
	tokens := strings.Split(driverUri, "/")
	return tmp_folder(driverUri) + tokens[len(tokens)-1]
}

var NOT_COMPRESSED  = errors.New("File not compressed.")

func pickDriverFileName(driverUri string) (string, error) {
	tokens := strings.Split(driverUri, "/")
	tokens = strings.Split(tokens[len(tokens)-1], ".")
	if len(tokens) == 1 {
		return "", NOT_COMPRESSED
	}
	if tokens[len(tokens) -1] == "zip" {
		return tmp_folder(driverUri) + tokens[len(tokens) - 2], nil
	}
	if tokens[len(tokens) -2] == "tar" {
		return tmp_folder(driverUri) + tokens[len(tokens) - 3], nil
	}
	if tokens[len(tokens) -1] == "tar" {
		return tmp_folder(driverUri) + tokens[len(tokens) - 2], nil
	}
	return "", errors.New(fmt.Sprintf("Can't pick filename for driver, not sure of compression: %s", driverUri))
}

func downloadVerifyExtractDriver(driverUri, driverMD5checksum, driverName string) (error) {
	fileName, err := downloadFromUrl(driverUri)
	if err != nil {
		return err
	}
	err = verifyCheckSum(fileName, driverMD5checksum)

	if err != nil {
		return err
	}

	_, err = pickDriverFileName(driverUri)

	if err == nil {
		fileName, err = extractDriver(driverUri)
		if err != nil {
			return err
		}
	} else if err != NOT_COMPRESSED {
		return err
	}

	return os.Rename(fileName, DRIVERS_LOCATION + asDockerMachineDriver(driverName))
}


func DownloadDrivers() ([]client.MachineDriver, []error, error) {
	var err error
	var apiClient *client.RancherClient
	errs := []error {}

	apiClient, err = getClient()

	if err != nil {
		return nil, errs, err
	}

	drivers, err := apiClient.MachineDriver.List(client.NewListOpts())

	if err != nil {
		return nil, errs, err
	}

	log.Debug("Starting parallel download of drivers.")

	var  wg sync.WaitGroup

	for _,driver := range drivers.Data {
		wg.Add(1)
		go func(driver client.MachineDriver) {
			defer wg.Done()

			reinstall := false
			handled := false

			if driver.State == "active" {
				log.Debug("Verfiying ", driver.Name, " exists in path.")
				reinstall = !driverBinaryExists(driver.Name)
				if reinstall {
					log.Info("Active driver ", driver.Name, " binary: ", asDockerMachineDriver(driver.Name),
						" not found reinstalling.")
				} else {
					log.Info("Active driver ", driver.Name, " currently installed at ",
						asDockerMachineDriver(driver.Name))
				}
			}

			if driver.State == "inactive" || reinstall {
				log.Debug("Downloading and verifying: " + driver.Uri)
				err := installDriver(driver.Uri, driver.Md5checksum, driver.Name)
				if err != nil {
					driver.TransitioningMessage = err.Error()
					apiClient.MachineDriver.ActionError(&driver)
					log.Error("Error while downloading and verifying: ", driver.Uri, err)
				} else {
					apiClient.MachineDriver.ActionActivate(&driver)
					log.Debug("Activating driver: ", driver.Name)
				}
				handled = true
			} else if driver.State == "error" || driver.State == "erroring" {
				log.Error("Driver: ", driver.Name, " is ", driver.State, " ignoring driver download.")
				handled = true
			} else if !handled {
				log.Debug("Driver: ", driver.Name, " is ", driver.State, " unknown state nothing was done.")
			}
			//Behavior if driver is active?
		}(driver)
	}

	wg.Wait()

	driversRefreshed, err := apiClient.MachineDriver.List(client.NewListOpts())

	if err != nil {
		return nil, errs, err
	}

	driverNames := localbinary.CoreDrivers[:]
	//Start with core drivers.

	for _,driver := range driversRefreshed.Data {
		if driver.State == "active" {
			//Only add active drivers in cattle. Inactive and erroring ones are ignored.
			driverNames = append(driverNames[:], driver.Name)
		}
	}

	for _,driver := range driverNames {
		wg.Add(1)
		go  func(driver string) {
			defer  wg.Done()
			log.Debug("Generating schema for: ", driver)
			err = generateAndUploadSchema(driver)
			if (err != nil){
				log.Debug("Err from routine:" + err.Error())
				errs = append(errs, err)
			}
		} (driver)
	}

	wg.Wait()

	return driversRefreshed.Data, errs, uploadMachineSchema(driverNames[:])
}

func downloadFromUrl(url string) (string, error) {
	fileName := pickDownloadFileName(url)
	log.Debug("Downloading: " + fileName)
	output, err := os.Create(fileName)
	if err != nil {
		return fileName, err
	}
	defer output.Close()

	response, err := http.Get(url)
	if err != nil {
		return fileName, err
	}
	defer response.Body.Close()

	_, err = io.Copy(output, response.Body)
	if err != nil {
		return fileName, err
	}
	return  fileName, nil
}

type InvalidCheckSum struct {
	driverFile, driverMD5, calculatedCheckSum string
}

func (e InvalidCheckSum) Error() string {
	return fmt.Sprintf("Checksum provided: %v does not match calculated checksum : %v for driver with uri: %v", e.driverMD5, e.calculatedCheckSum, e.driverFile)
}

func verifyCheckSum(fileName, driverMD5 string) error {
	if  driverMD5 == "" {
		log.Debug("No md5 skipping check.")
		return nil
	}
	log.Debug("Checking md5 hash: " + fileName)
	checkSumCalculated, err := computeMd5(fileName)
	if err != nil {
		return err
	}
	if checkSumCalculated != driverMD5 {
		log.Error("Failed checkSum check: " + fileName)
		log.Debug("Calced: " + checkSumCalculated + " given: " + driverMD5)
		return InvalidCheckSum{
			fileName,
			driverMD5,
			checkSumCalculated,
		}
	}
	return nil
}

const DRIVERS_LOCATION = "/usr/local/bin/"

func installDriver(driverUri, driverMD5, driverName string) error {

	err := downloadVerifyExtractDriver(driverUri, driverMD5, driverName)
	if err != nil  {
		return err
	}

	return os.Chmod(DRIVERS_LOCATION + asDockerMachineDriver(driverName), 0777)
}

func asDockerMachineDriver(driverName string) string {
	return "docker-machine-driver-" + strings.TrimPrefix(driverName, "docker-machine-driver-")
}

type MultipleFiles struct {
	folder string
}

func (e MultipleFiles) Error() string {
	return fmt.Sprintf("Multiple files with docker-machine-driver in %v", e.folder)
}

func extractDriver(driverUri string) (string, error) {
	_, err := pickDriverFileName(driverUri)
	if err != nil {
		return "", err
	}

	fileName := pickDownloadFileName(driverUri)
	log.Debug("Extracting... ", fileName)
	tar, err := exec.LookPath("tar")
	if err != nil {
		return "", err
	}
	temp_folder := tmp_folder(driverUri + fileName)
	extraction := exec.Command(tar, "-xvf", fileName, "-C", temp_folder)
	output, err := extraction.CombinedOutput()
	log.Debug(string(output[:]))
	if err != nil {
		return "", err
	}
	files, err := ioutil.ReadDir(temp_folder)
	var fileNames []string
	for _,file := range files {
		if !file.IsDir() {
			if strings.HasPrefix(file.Name(), "docker-machine-driver") {
				fileNames = append(fileNames, temp_folder + file.Name())
			}
		}
	}
	if len(fileNames) > 1 {
		return "", MultipleFiles{ temp_folder }
	}
	return fileNames[0], err
}

func driverBinaryExists(driverName string) (bool) {
	_, err := exec.LookPath(asDockerMachineDriver(driverName))
	return err == nil
}
