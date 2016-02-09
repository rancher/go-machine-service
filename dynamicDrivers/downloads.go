package dynamicDrivers

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/machine/libmachine/drivers/plugin/localbinary"
	"github.com/rancher/go-rancher/client"
	"hash/fnv"
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

func tmpFolder(uri string) string {
	h := fnv.New32a()
	h.Write([]byte(uri))
	var folderName = fmt.Sprintf("%v%v%v", "/tmp/", h.Sum32(), "/")
	if _, err := os.Stat(folderName); os.IsNotExist(err) {
		os.MkdirAll(folderName, 0777)
	}
	return folderName
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

func pickDownloadFileName(driverURI string) string {
	tokens := strings.Split(driverURI, "/")
	return tmpFolder(driverURI) + tokens[len(tokens)-1]
}

const (
	none               = Compression("File not compressed.")
	unKnownCompression = "File compression unknown."
	tar                = "TAR"
	zip                = "ZIP"
)

var errNotCompressed = errors.New("File not compressed.")

type Compression string

func getCompression(driverURI string) Compression {
	tokens := strings.Split(driverURI, "/")
	tokens = strings.Split(tokens[len(tokens)-1], ".")[1:]
	if len(tokens) == 0 {
		return none
	}
	if tokens[0] == "zip" && len(tokens) == 1 {
		return zip
	}
	if tokens[0] == "tar" {
		return tar
	}
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

	compression := getCompression(driverURI)

	if compression == unKnownCompression {
		return errors.New(driverName + " compression unknown.")
	} else if compression != none {
		fileName, err = extractDriver(driverURI)
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

	driverNames := localbinary.CoreDrivers[:]
	//Start with core drivers.

	driversMap := make(map[string]client.MachineDriver)
	for _, driver := range driversRefreshed.Data {
		if driver.State == "active" {
			//Only add active drivers in cattle. Inactive and Erroring ones are ignored.
			driverNames = append(driverNames, driver.Name)
			driversMap[driver.Name] = driver
		} else {
			removeSchema(driver.Name, apiClient)
		}
	}

	errsChan := make(chan []error)
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
				if val, ok := driversMap[driver]; ok {
					input := client.MachineDriverErrorInput{ErrorMessage: errFunc.Error()}
					_, errFunc = apiClient.MachineDriver.ActionError(&val, &input)
					if errFunc != nil {
						routineErrors = append(routineErrors, err)
					}
					errFunc = waitSuccessDriver(val, apiClient)
					if errFunc != nil {
						routineErrors = append(routineErrors, err)
					}
				}
			}
			errsChan <- routineErrors
		}(driver)
	}

	for range driverNames {
		allErrors = append(allErrors, <-errsChan...)
	}

	wg.Wait()

	return driversRefreshed.Data, append(allErrors, uploadMachineSchema(driverNames))
}

func downloadFromURL(url string) (string, error) {
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

func extractDriver(driverURI string) (string, error) {
	compression := getCompression(driverURI)
	if compression == none {
		return "", errNotCompressed
	}

	fileName := pickDownloadFileName(driverURI)
	log.Debug("Extracting... ", fileName)
	tempFolder := tmpFolder(driverURI + fileName)
	if compression == zip {
		unzip, err := exec.LookPath("unzip")
		if err != nil {
			return "", err
		}
		extraction := exec.Command(unzip, fileName, "-d", tempFolder)
		output, err := extraction.CombinedOutput()
		log.Debug(string(output[:]))
		if err != nil {
			return "", err
		}
	} else if compression == tar {
		tar, err := exec.LookPath("tar")
		if err != nil {
			return "", err
		}
		extraction := exec.Command(tar, "-xvf", fileName, "-C", tempFolder)
		output, err := extraction.CombinedOutput()
		log.Debug(string(output[:]))
		if err != nil {
			return "", err
		}
	} else {
		return "", errors.New(fileName + " compression unknown.")
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
	return fileNames[0], err
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

	if driver.State == "inactive" || reinstall {
		log.Debug("Downloading and verifying: " + driver.Uri)
		err := installDriver(driver.Uri, driver.Md5checksum, driver.Name)
		if err != nil {
			input := client.MachineDriverErrorInput{ErrorMessage: err.Error()}
			apiClient.MachineDriver.ActionError(&driver, &input)
			log.Error("Error while downloading and verifying: ", driver.Uri, err)
			err = waitSuccessDriver(driver, apiClient)
			if err != nil {
				log.Error("Error waiting for driver to error:", err)
			}
		} else {
			apiClient.MachineDriver.ActionActivate(&driver)
			log.Debug("Activating driver: ", driver.Name)
			err = waitSuccessDriver(driver, apiClient)
			if err != nil {
				log.Error("Error waiting for driver to activate:", err)
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
