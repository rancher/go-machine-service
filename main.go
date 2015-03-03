package main

import (
	"flag"
	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-machine-service/handlers"
	"os"
)

func main() {
	processCmdLineFlags()

	log.Info("Starting go-machine-service...")
	eventHandlers := map[string]events.EventHandler{
		"physicalhost.create":    handlers.CreateMachine,
		"physicalhost.bootstrap": handlers.ActivateMachine,
		"physicalhost.remove":    handlers.PurgeMachine,
		"ping":                   handlers.PingNoOp,
	}

	apiUrl := os.Getenv("CATTLE_URL")
	accessKey := os.Getenv("CATTLE_ACCESS_KEY")
	secretKey := os.Getenv("CATTLE_SECRET_KEY")

	router, err := events.NewEventRouter("goMachineService", 2000, apiUrl, accessKey, secretKey,
		nil, eventHandlers, 10)
	if err != nil {
		log.WithFields(log.Fields{
			"Err": err,
		}).Error("Unable to create EventRouter")
	} else {
		router.Start(nil)
	}
	log.Info("Exiting go-machine-service...")
}

func processCmdLineFlags() {
	// Define command line flags
	logLevel := flag.String("loglevel", "info", "Set the default loglevel (default:info) [debug|info|warn|error]")

	flag.Parse()

	// Process log level.  If an invalid level is passed in, we simply default to info.
	if parsedLogLevel, err := log.ParseLevel(*logLevel); err == nil {
		log.WithFields(log.Fields{
			"logLevel": *logLevel,
		}).Info("Setting log level")
		log.SetLevel(parsedLogLevel)
	}
}
