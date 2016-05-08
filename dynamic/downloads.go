package dynamic

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	"github.com/rancher/go-rancher/client"
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

func UpdateDrivers() []error {
	log.Info("Updating docker-machine-drivers from cattle.")
	if _, err := os.Stat(DriversLocation); os.IsNotExist(err) {
		err = os.MkdirAll(DriversLocation, 0777)
		if err != nil {
			return []error{err}
		}
		log.Debug("Made folder: ", DriversLocation)
	}

	apiClient, err := getClient()

	if err != nil {
		return []error{err}
	}

	_, errs := downloadDrivers(apiClient)
	if len(errs) > 0 {
		for _, err = range errs {
			if err != nil {
				return errs
			}
		}
	}

	time.Sleep(5 * time.Second)
	driversCollection, err := apiClient.MachineDriver.List(nil)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	blackListSetting, err := getBlackListSetting(apiClient)
	if err != nil {
		errs = append(errs, err)
		return errs
	}

	go killOnChange(driversCollection.Data, blackListSetting.Value, apiClient)
	return errs
}

func killOnChange(startingDrivers []client.MachineDriver, blackListedDrivers string, apiClient *client.RancherClient) {
	reconnectAttemps := 0
	log.Info("Started setting and driver watcher.")
	for {
		drivers, err := apiClient.MachineDriver.List(nil)
		if err == nil {
			if !reflect.DeepEqual(drivers.Data, startingDrivers) {
				log.Info("Detected change to rancher defined machine drivers. Exiting go machine service.")
				os.Exit(0)
			}
			blackListSetting, err := getBlackListSetting(apiClient)
			if err == nil {
				if blackListSetting.Value != blackListedDrivers {
					log.Info("Detected change to rancher driver blacklist. Exiting go machine service.")
					os.Exit(0)
				}
				time.Sleep(time.Second * 5)
				reconnectAttemps = 0
			} else {
				time.Sleep(getTime(&reconnectAttemps))
			}
		} else {
			time.Sleep(getTime(&reconnectAttemps))
		}
	}
}

func getTime(reconnectAttempts *int) time.Duration {
	*reconnectAttempts = *reconnectAttempts + 1
	totalTime := math.Pow(2, float64(*reconnectAttempts))
	if totalTime <= 64 {
		log.Info("Failed to connect", *reconnectAttempts, " times.")
		return time.Duration(totalTime) * time.Second
	}
	log.Fatal("Failed to connect to cattle. ", *reconnectAttempts, " Exiting ", os.Args[0])
	return 60 * time.Second
}

const (
	none               = Compression("File not compressed.")
	unKnownCompression = "File compression unknown."
	tar                = "TAR"
	zip                = "ZIP"
)

var errNotCompressed = errors.New("File not compressed.")

type Compression string

func getCompression(fileName string) Compression {
	log.Debugf("Driver uri: %v", fileName)
	tokens := strings.Split(fileName, "/")
	log.Debugf("Driver tokens on / : %v", tokens)
	tokens = strings.Split(tokens[len(tokens)-1], ".")[1:]
	log.Debugf("Driver tokens on . : %v", tokens)
	if len(tokens) == 0 {
		log.Debugf("Driver %v is %v", fileName, none)
		return none
	}
	if tokens[len(tokens)-1] == "zip" {
		log.Debugf("Driver %v is %v", fileName, zip)
		return zip
	}
	if tokens[len(tokens)-1] == "tar" || tokens[len(tokens)-2] == "tar" {
		log.Debugf("Driver %v is %v", fileName, tar)
		return tar
	}
	log.Debugf("Unknown compression: %#v", tokens)
	return unKnownCompression
}

func downloadVerifyExtractDriver(driverURI, driverMD5checksum, driverName string) error {
	fileName, err := downloadFromURL(driverURI)
	if err != nil {
		return err
	}
	err = verifyCheckSum(fileName, driverMD5checksum)

	if err != nil {
		return err
	}

	compression := getCompression(fileName)

	if compression == unKnownCompression {
		return errors.New(driverName + " compression unknown.")
	} else if compression != none {
		fileName, err = extractDriver(fileName)
		if err != nil {
			return err
		}
	}
	return os.Rename(fileName, DriversLocation+asDockerMachineDriver(driverName))
}

func downloadDrivers(apiClient *client.RancherClient) ([]client.MachineDriver, []error) {
	var err error
	allErrors := []error{}

	drivers, err := apiClient.MachineDriver.List(nil)

	if err != nil {
		allErrors = append(allErrors, err)
		return nil, allErrors
	}

	blackList, err := getBlackListedDrivers(apiClient)
	if err != nil {
		allErrors = append(allErrors, err)
		return nil, allErrors
	}

	log.Debug("Starting parallel download of drivers.")

	var wg sync.WaitGroup

	for _, driver := range drivers.Data {
		wg.Add(1)
		go downloadMachineDriver(driver, blackList, &wg, apiClient)
	}

	wg.Wait()

	driversRefreshed, err := apiClient.MachineDriver.List(nil)

	if err != nil {
		allErrors = append(allErrors, err)
		return nil, allErrors
	}

	dNames := localbinary.CoreDrivers[:]
	//Start with core drivers.

	driverNames := []string{}
	for _, d := range dNames {
		if d == "none" {
			continue
		}
		driverNames = append(driverNames, d)
	}

	driversMap := make(map[string]client.MachineDriver)
	for _, driver := range driversRefreshed.Data {
		if driver.State == "requested" || driver.State == "active" {
			//Only add active and requested drivers in cattle. Inactive and Erroring ones are ignored.
			driverNames = append(driverNames, driver.Name)
			driversMap[driver.Name] = driver
		} else {
			log.Info(driver.Name, " not to be used removing any schemas it has.")
			removeSchema(driver.Name+"Confg", apiClient)
		}
	}

	errsChan := make(chan []error)
	driversPublished := []string{}
	for _, driver := range driverNames {
		wg.Add(1)
		go func(driver string) {
			defer wg.Done()
			routineErrors := []error{}
			if isBlacklisted(blackList, driver) {
				log.Info(driver, " is blacklisted removing any schemas it has.")
				err := removeSchema(driver+"Config", apiClient)
				if err != nil {
					routineErrors = append(routineErrors, err)
				}
				errsChan <- routineErrors
				return
			}
			log.Debug("Generating schema for: ", driver)
			errFunc := generateAndUploadSchema(driver)
			if errFunc != nil {
				routineErrors = append(routineErrors, errFunc)
			}
			if cattleDriverResource, ok := driversMap[driver]; ok {
				if errFunc != nil {
					input := client.MachineDriverErrorInput{ErrorMessage: errFunc.Error()}
					_, errFunc = apiClient.MachineDriver.ActionError(&cattleDriverResource, &input)
					if errFunc != nil {
						routineErrors = append(routineErrors, errFunc)
					}
					errFunc = waitSuccessDriver(cattleDriverResource, apiClient)
					if errFunc != nil {
						routineErrors = append(routineErrors, errFunc)
					}
				}
			}
			errored := false
			if len(routineErrors) > 0 {
				for _, err := range routineErrors {
					if err != nil {
						errored = true
					}
				}
			}
			if !errored {
				driversPublished = append(driversPublished, driver)
			}
			errsChan <- routineErrors
		}(driver)
	}

	for range driverNames {
		allErrors = append(allErrors, <-errsChan...)
	}

	wg.Wait()

	if len(allErrors) > 0 {
		for _, err := range allErrors {
			if err != nil {
				return nil, allErrors
			}
		}
	}

	err = uploadMachineSchema(driverNames)
	if err != nil {
		return nil, []error{err}
	}

	for _, driver := range driversPublished {
		go func(driver string) {
			routineErrors := []error{}
			defer func() { errsChan <- routineErrors }()
			if cattleDriverResource, ok := driversMap[driver]; ok {
				if cattleDriverResource.State != "active" {
					_, errFunc := apiClient.MachineDriver.ActionActivate(&cattleDriverResource)
					if errFunc != nil {
						routineErrors = append(routineErrors, errFunc)
					}
					log.Debug("Activating driver: ", cattleDriverResource.Name)
					err = waitSuccessDriver(cattleDriverResource, apiClient)
					if err != nil {
						log.Error("Error waiting for driver to activate:", err)
						routineErrors = append(routineErrors, err)
					}
				}
			}
		}(driver)
	}

	for range driversPublished {
		allErrors = append(allErrors, <-errsChan...)
	}

	wg.Wait()

	driversRefreshed, err = apiClient.MachineDriver.List(nil)
	if err != nil {
		allErrors = append(allErrors, err)
		return nil, allErrors
	}

	return driversRefreshed.Data, allErrors
}

func downloadFromURL(url string) (string, error) {
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	tmpFolder, err := ioutil.TempDir("", "gms-")
	if err != nil {
		return "", err
	}
	tmpFolder = tmpFolder + "/"
	log.Debug("Downloading: %v to folder %v", fileName, tmpFolder)
	fileName = tmpFolder + fileName

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
	return fileName, nil
}

type InvalidCheckSum struct {
	driverFile, driverMD5, calculatedCheckSum string
}

func (e InvalidCheckSum) Error() string {
	return fmt.Sprintf("Checksum provided: %v does not match calculated checksum : %v for driver with uri: %v", e.driverMD5, e.calculatedCheckSum, e.driverFile)
}

func verifyCheckSum(fileName, driverMD5 string) error {
	if driverMD5 == "" {
		log.Debug("No md5 skipping check.")
		return nil
	}
	log.Debug("Checking md5 hash: " + fileName)
	checkSumCalculated, err := computeMd5(fileName)
	if err != nil {
		return err
	}
	if checkSumCalculated != driverMD5 {
		return InvalidCheckSum{
			fileName,
			driverMD5,
			checkSumCalculated,
		}
	}
	return nil
}

const DriversLocation = "/usr/local/bin/"

func installDriver(driverURI, driverMD5, driverName string) error {

	err := downloadVerifyExtractDriver(driverURI, driverMD5, driverName)
	if err != nil {
		return err
	}

	return os.Chmod(DriversLocation+asDockerMachineDriver(driverName), 0755)
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

func extractDriver(fileName string) (string, error) {
	compression := getCompression(fileName)
	if compression == none {
		return "", errNotCompressed
	}

	tempFolder, err := ioutil.TempDir("", "gms-")
	if err != nil {
		return fileName, err
	}
	tempFolder = tempFolder + "/"
	log.Debug("Extracting... ", fileName)
	var extraction *exec.Cmd
	if compression == zip {
		unzip, err := exec.LookPath("unzip")
		if err != nil {
			return "", err
		}
		extraction = exec.Command(unzip, "-o", fileName, "-d", tempFolder)
	} else if compression == tar {
		tar, err := exec.LookPath("tar")
		if err != nil {
			return "", err
		}
		extraction = exec.Command(tar, "-xvf", fileName, "-C", tempFolder)
	} else {
		return "", errors.New(fileName + " compression unknown.")
	}
	log.Debugf("Command for %v : %v", fileName, extraction.Args)
	output, err := extraction.CombinedOutput()
	log.Debug(string(output[:]))
	if err != nil {
		return "", err
	}

	files, err := ioutil.ReadDir(tempFolder)
	var fileNames []string
	for _, file := range files {
		if !file.IsDir() {
			if strings.HasPrefix(file.Name(), "docker-machine-driver") {
				fileNames = append(fileNames, tempFolder+file.Name())
			}
		}
	}
	if len(fileNames) > 1 {
		return "", MultipleFiles{tempFolder}
	}
	return fileNames[0], nil
}

func driverBinaryExists(driverName string) bool {
	_, err := exec.LookPath(asDockerMachineDriver(driverName))
	return err == nil
}

func downloadMachineDriver(driver client.MachineDriver, blackList []string,
	wg *sync.WaitGroup, apiClient *client.RancherClient) {
	defer wg.Done()
	if isBlacklisted(blackList, driver.Name) {
		log.Info("Driver: ", driver.Name, " is ", driver.State, " but was in blacklist. Ignoring.")
		return
	}

	reinstall := false
	handled := false

	if driver.State == "active" {
		log.Debug("Verfiying ", driver.Name, " exists in path.")
		reinstall = !driverBinaryExists(driver.Name)
		if reinstall {
			log.Info("Active driver ", driver.Name, " binary, ", asDockerMachineDriver(driver.Name),
				", not found reinstalling.")
		} else {
			log.Info("Active driver ", driver.Name, " currently installed at ",
				asDockerMachineDriver(driver.Name), ".")
			handled = true
		}
	}

	if driver.State == "requested" || reinstall {
		log.Debug("Downloading and verifying: " + driver.Uri)
		err := installDriver(driver.Uri, driver.Md5checksum, driver.Name)
		if err != nil {
			input := client.MachineDriverErrorInput{ErrorMessage: err.Error()}
			apiClient.MachineDriver.ActionError(&driver, &input)
			log.Errorf("Error while downloading and verifying: %v %v", driver.Uri, err)
			err = waitSuccessDriver(driver, apiClient)
			if err != nil {
				log.Error("Error waiting for driver to error:", err)
			}
		}
		handled = true
	} else if driver.State == "error" || driver.State == "erroring" {
		log.Error("Driver: ", driver.Name, " is ", driver.State, " ignoring driver download.")
		handled = true
	}

	if !handled {
		log.Warn("Driver: ", driver.Name, " is ", driver.State, " unknown state nothing was done.")
	}
}
