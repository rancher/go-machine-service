package main

import (
	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-machine-service/handlers"
	"os"
)

func main() {
	// Process options
	for _, f := range os.Args {
		if f == "-D" || f == "--debug" || f == "-debug" {
			log.SetLevel(log.DebugLevel)
		}
	}

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
