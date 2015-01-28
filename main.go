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

	apiUrl := utils.GetRancherUrl(false) + "/v1"
	router := events.NewEventRouter("goMachineService", 2000, apiUrl, eventHandlers, 3)
	router.Start(nil)
	log.Println("Leaving go-machine-service...")
}
