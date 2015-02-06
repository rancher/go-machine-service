package main

import (
	"github.com/rancherio/go-machine-service/events"
	"github.com/rancherio/go-machine-service/handlers"
	"github.com/rancherio/go-machine-service/utils"
	"log"
)

func main() {
	log.Println("Beginning go-machine-service...")
	eventHandlers := map[string]events.EventHandler{
		"physicalhost.create":    handlers.CreateMachine,
		"physicalhost.bootstrap": handlers.ActivateMachine,
		"physicalhost.remove":    handlers.PurgeMachine,
		"ping":                   handlers.PingNoOp,
	}

	apiUrl := utils.GetRancherUrl(false)
	accessKey := utils.GetRancherAccessKey()
	secretKey := utils.GetRancherSecretKey()

	router, err := events.NewEventRouter("goMachineService", 2000, apiUrl, accessKey, secretKey,
		nil, eventHandlers, 10)
	if err != nil {
		log.Println("Unable to create EventRouter", err)
	} else {
		router.Start(nil)
	}
	log.Println("Leaving go-machine-service...")
}
