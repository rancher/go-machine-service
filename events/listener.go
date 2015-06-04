package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/rancherio/go-machine-service/locks"
	"github.com/rancherio/go-rancher/client"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const MaxWait = time.Duration(time.Second * 10)

// Defines the function "interface" that handlers must conform to.
type EventHandler func(*Event, *client.RancherClient) error

type EventRouter struct {
	name                string
	priority            int
	apiUrl              string
	accessKey           string
	secretKey           string
	apiClient           *client.RancherClient
	subscribeUrl        string
	eventHandlers       map[string]EventHandler
	workerCount         int
	eventStreamResponse *http.Response
}

type ProcessConfig struct {
	Name    string `json:"name"`
	OnError string `json:"onError"`
}

func (router *EventRouter) Start(ready chan<- bool) (err error) {
	workers := make(chan *Worker, router.workerCount)
	for i := 0; i < router.workerCount; i++ {
		w := newWorker()
		workers <- w
	}

	log.WithFields(log.Fields{
		"workerCount": router.workerCount,
	}).Info("Initializing event router")

	// If it exists, delete it, then create it
	err = removeOldHandler(router.name, router.apiClient)
	if err != nil {
		return err
	}

	externalHandler := &client.ExternalHandler{
		Name:           router.name,
		Uuid:           router.name,
		Priority:       int64(router.priority),
		ProcessConfigs: make([]interface{}, len(router.eventHandlers)),
	}

	handlers := map[string]EventHandler{}

	if pingHandler, ok := router.eventHandlers["ping"]; ok {
		// Ping doesnt need registered in the POST and ping events don't have the handler suffix.
		//If we start handling other non-suffix events, we might consider improving this.
		handlers["ping"] = pingHandler
	}

	idx := 0
	subscribeForm := url.Values{}
	eventHandlerSuffix := ";handler=" + router.name
	for event, handler := range router.eventHandlers {
		processConfig := ProcessConfig{
			Name:    event,
			OnError: "physicalhost.error",
		}
		externalHandler.ProcessConfigs[idx] = processConfig
		fullEventKey := event + eventHandlerSuffix
		subscribeForm.Add("eventNames", fullEventKey)
		handlers[fullEventKey] = handler
		idx++
	}
	err = createNewHandler(externalHandler, router.apiClient)
	if err != nil {
		return err
	}

	if ready != nil {
		ready <- true
	}

	eventStream, err := subscribeToEvents(router.subscribeUrl, router.accessKey, router.secretKey, subscribeForm)
	if err != nil {
		return err
	}
	log.Info("Connection established")
	router.eventStreamResponse = eventStream
	defer eventStream.Body.Close()

	scanner := bufio.NewScanner(eventStream.Body)
	for scanner.Scan() {
		line := scanner.Bytes()

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		select {
		case worker := <-workers:
			go worker.DoWork(line, handlers, router.apiClient, workers)
		default:
			log.WithFields(log.Fields{
				"workerCount": router.workerCount,
			}).Info("No workers available dropping event.")
		}
	}

	return nil
}

func (router *EventRouter) Stop() (err error) {
	router.eventStreamResponse.Body.Close()
	return nil
}

// TODO Privatize worker
type Worker struct {
}

func (w *Worker) DoWork(rawEvent []byte, eventHandlers map[string]EventHandler, apiClient *client.RancherClient,
	workers chan *Worker) {
	defer func() { workers <- w }()

	event := &Event{}
	err := json.Unmarshal(rawEvent, &event)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Error unmarshalling event")
		return
	}

	if event.Name != "ping" {
		log.WithFields(log.Fields{
			"event": string(rawEvent[:]),
		}).Debug("Processing event.")
	}

	unlocker := locks.Lock(event.ResourceId)
	if unlocker == nil {
		log.WithFields(log.Fields{
			"resourceId": event.ResourceId,
		}).Debug("Resource locked. Dropping event")
		return
	}
	defer unlocker.Unlock()

	if fn, ok := eventHandlers[event.Name]; ok {
		err = fn(event, apiClient)
		if err != nil {
			log.WithFields(log.Fields{
				"eventName":  event.Name,
				"eventId":    event.Id,
				"resourceId": event.ResourceId,
				"err":        err,
			}).Error("Error processing event")

			reply := &client.Publish{
				Name:                 event.ReplyTo,
				PreviousIds:          []string{event.Id},
				Transitioning:        "error",
				TransitioningMessage: err.Error(),
			}
			_, err := apiClient.Publish.Create(reply)
			if err != nil {
				log.WithFields(log.Fields{
					"err": err,
				}).Error("Error sending error-reply")
			}
		}
	} else {
		log.WithFields(log.Fields{
			"eventName": event.Name,
		}).Warn("No event handler registered for event")
	}
}

func NewEventRouter(name string, priority int, apiUrl string, accessKey string, secretKey string,
	apiClient *client.RancherClient, eventHandlers map[string]EventHandler, workerCount int) (*EventRouter, error) {

	if apiClient == nil {
		var err error
		apiClient, err = client.NewRancherClient(&client.ClientOpts{

			Url:       apiUrl,
			AccessKey: accessKey,
			SecretKey: secretKey,
		})
		if err != nil {
			return nil, err
		}
	}

	return &EventRouter{
		name:          name,
		priority:      priority,
		apiUrl:        apiUrl,
		accessKey:     accessKey,
		secretKey:     secretKey,
		apiClient:     apiClient,
		subscribeUrl:  apiUrl + "/subscribe",
		eventHandlers: eventHandlers,
		workerCount:   workerCount,
	}, nil
}

func newWorker() *Worker {
	return &Worker{}
}

var subscribeToEvents = func(subscribeUrl string, user string, pass string, data url.Values) (resp *http.Response, err error) {
	subscribeClient := &http.Client{}
	req, err := http.NewRequest("POST", subscribeUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(user, pass)
	return subscribeClient.Do(req)
}

var createNewHandler = func(externalHandler *client.ExternalHandler, apiClient *client.RancherClient) error {
	_, err := apiClient.ExternalHandler.Create(externalHandler)
	return err
}

var removeOldHandler = func(name string, apiClient *client.RancherClient) error {
	listOpts := client.NewListOpts()
	listOpts.Filters["name"] = name
	listOpts.Filters["state"] = "active"
	handlers, err := apiClient.ExternalHandler.List(listOpts)
	if err != nil {
		return err
	}

	for _, handler := range handlers.Data {
		h := &handler
		log.WithFields(log.Fields{
			"handlerId": h.Id,
		}).Debug("Removing old handler")
		doneTransitioning := func() (bool, error) {
			h, err := apiClient.ExternalHandler.ById(h.Id)
			if err != nil {
				return false, err
			}
			return h.Transitioning != "yes", nil
		}

		if _, ok := h.Actions["deactivate"]; ok {
			h, err = apiClient.ExternalHandler.ActionDeactivate(h)
			if err != nil {
				return err
			}

			err = waitForTransition(doneTransitioning)
			if err != nil {
				return err
			}
		}

		h, err := apiClient.ExternalHandler.ById(h.Id)
		if err != nil {
			return err
		}
		if _, ok := h.Actions["remove"]; ok {
			h, err = apiClient.ExternalHandler.ActionRemove(h)
			if err != nil {
				return err
			}
			err = waitForTransition(doneTransitioning)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type doneTranitioningFunc func() (bool, error)

func waitForTransition(waitFunc doneTranitioningFunc) error {
	timeoutAt := time.Now().Add(MaxWait)
	ticker := time.NewTicker(time.Millisecond * 250)
	defer ticker.Stop()
	for tick := range ticker.C {
		done, err := waitFunc()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if tick.After(timeoutAt) {
			return fmt.Errorf("Timed out waiting for transtion.")
		}
	}
	return fmt.Errorf("Timed out waiting for transtion.")
}
