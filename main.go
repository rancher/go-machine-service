package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rancher/event-subscriber/events"
	"github.com/rancher/go-machine-service/dynamic"
	"github.com/rancher/go-machine-service/handlers"
	"github.com/rancher/go-machine-service/logging"
)

var (
	GITCOMMIT = "HEAD"
)

var logger = logging.Logger()

func main() {
	processCmdLineFlags()

	logger.WithField("gitcommit", GITCOMMIT).Info("Starting go-machine-service...")

	apiURL := os.Getenv("CATTLE_URL")
	accessKey := os.Getenv("CATTLE_ACCESS_KEY")
	secretKey := os.Getenv("CATTLE_SECRET_KEY")

	ready := make(chan bool, 2)
	done := make(chan error)

	go func() {
		eventHandlers := map[string]events.EventHandler{
			"machinedriver.reactivate": handlers.ActivateDriver,
			"machinedriver.activate":   handlers.ActivateDriver,
			"machinedriver.update":     handlers.ActivateDriver,
			"machinedriver.error":      handlers.ErrorDriver,
			"machinedriver.deactivate": handlers.DeactivateDriver,
			"machinedriver.remove":     handlers.RemoveDriver,
			"ping":                     handlers.PingNoOp,
		}

		router, err := events.NewEventRouter("goMachineService-machine", 2000, apiURL, accessKey, secretKey,
			nil, eventHandlers, "machineDriver", 250, events.DefaultPingConfig)
		if err == nil {
			err = router.Start(ready)
		}
		done <- err
	}()

	go func() {
		eventHandlers := map[string]events.EventHandler{
			"physicalhost.create":    handlers.CreateMachine,
			"physicalhost.bootstrap": handlers.ActivateMachine,
			"physicalhost.remove":    handlers.PurgeMachine,
			"ping":                   handlers.PingNoOp,
		}

		router, err := events.NewEventRouter("goMachineService", 2000, apiURL, accessKey, secretKey,
			nil, eventHandlers, "physicalhost", 250, events.DefaultPingConfig)
		if err == nil {
			err = router.Start(ready)
		}
		done <- err
	}()

	go func() {
		// Can not remove this as nothing will delete the handler entries
		eventHandlers := map[string]events.EventHandler{
			"ping": handlers.PingNoOp,
		}

		router, err := events.NewEventRouter("goMachineService-agent", 2000, apiURL, accessKey, secretKey,
			nil, eventHandlers, "agent", 5, events.DefaultPingConfig)
		if err == nil {
			err = router.Start(ready)
		}
		done <- err
	}()

	go func() {
		logger.Infof("Waiting for handler registration (1/2)")
		<-ready
		logger.Infof("Waiting for handler registration (2/2)")
		<-ready
		if err := dynamic.ReactivateOldDrivers(); err != nil {
			logger.Fatalf("Error reactivating old drivers: %v", err)
		}
		if err := dynamic.DownloadAllDrivers(); err != nil {
			logger.Fatalf("Error updating drivers: %v", err)
		}
	}()

	err := <-done
	if err == nil {
		logger.Infof("Exiting go-machine-service")
	} else {
		logger.Fatalf("Exiting go-machine-service: %v", err)
	}
}

func processCmdLineFlags() {
	// Define command line flags
	version := flag.Bool("v", false, "read the version of the go-machine-service")
	flag.Parse()
	if *version {
		fmt.Printf("go-machine-service\t gitcommit=%s\n", GITCOMMIT)
		os.Exit(0)
	}
}
