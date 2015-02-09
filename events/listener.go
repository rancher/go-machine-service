package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rancherio/go-machine-service/locks"
	"github.com/rancherio/go-rancher/client"
	"log"
	"net/http"
	"net/url"
	"time"
)

const MaxWait = time.Duration(time.Second * 10)

type ReplyEventHandler func(*ReplyEvent)

// Defines the function "interface" that handlers must conform to.
type EventHandler func(*Event, ReplyEventHandler, *client.RancherClient) error

type EventRouter struct {
	name                string
	priority            int
	apiUrl              string
	accessKey           string
	secretKey           string
	apiClient           *client.RancherClient
	subscribeUrl        string
	replyUrl            string
	eventHandlers       map[string]EventHandler
	workerCount         int
	eventStreamResponse *http.Response
}

func (router *EventRouter) Start(ready chan<- bool) (err error) {
	workers := make(chan *Worker, router.workerCount)
	for i := 0; i < router.workerCount; i++ {
		w := newWorker(router.replyUrl)
		workers <- w
	}

	// If it exists, delete it, then create it
	err = removeOldHandler(router.name, router.apiClient)
	if err != nil {
		return err
	}

	externalHandler := &client.ExternalHandler{
		Name:         router.name,
		Uuid:         router.name,
		Priority:     router.priority,
		ProcessNames: make([]string, len(router.eventHandlers)),
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
		externalHandler.ProcessNames[idx] = event
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

	eventStream, err := subscribeToEvents(router.subscribeUrl, subscribeForm)
	if err != nil {
		return err
	}
	log.Println("Connection established.")
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
			go worker.DoWork(line, router.replyHandler, handlers, router.apiClient, workers)
		default:
			log.Printf("No workers available dropping event.")
		}
	}

	return nil
}

func (router *EventRouter) Stop() (err error) {
	router.eventStreamResponse.Body.Close()
	return nil
}

func (r *EventRouter) replyHandler(replyEvent *ReplyEvent) {
	// TODO Is passing this function into a goroutine non-idiomatic?
	log.Printf("Replying to [%v] with event [%v]", r.replyUrl, replyEvent)

	replyEventJson, err := json.Marshal(replyEvent)
	if err != nil {
		log.Printf("Can't marshal event. Error: %v.", err)
		return
	}

	eventBuffer := bytes.NewBuffer(replyEventJson)
	replyRequest, err := http.NewRequest("POST", r.replyUrl, eventBuffer)
	replyRequest.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(replyRequest)
	if err != nil {
		log.Printf("Can't send reply event. Error: %v. Returning", err)
		return
	}
	defer response.Body.Close()
	log.Printf("Replied to event: %v. Response code: %s", replyEvent.Name, response.Status)
}

// TODO Privatize worker
type Worker struct {
}

func (w *Worker) DoWork(rawEvent []byte, replyEventHandler ReplyEventHandler,
	eventHandlers map[string]EventHandler, apiClient *client.RancherClient, workers chan *Worker) {
	defer func() { workers <- w }()

	event := &Event{}
	err := json.Unmarshal(rawEvent, &event)
	if err != nil {
		log.Printf("Error unmarshalling event: %v", err)
		return
	}

	log.Printf("Received event: %v", event.Name)
	unlocker := locks.Lock(event.ResourceId)
	if unlocker == nil {
		log.Printf("Resource [%v] locked. Dropping event.", event.ResourceId)
		return
	}
	defer unlocker.Unlock()

	if fn, ok := eventHandlers[event.Name]; ok {
		err = fn(event, replyEventHandler, apiClient)
		if err != nil {
			log.Printf("Error processing event. Event name: %v. Event id: %v Resource id: %v. Error: %v",
				event.Name, event.Id, event.ResourceId, err)
		}
	} else {
		log.Printf("No handler registered for event %v", event.Name)
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
		replyUrl:      apiUrl + "/publish",
		eventHandlers: eventHandlers,
		workerCount:   workerCount,
	}, nil
}

func newWorker(replyUrl string) *Worker {
	return &Worker{}
}

var subscribeToEvents = func(subscribeUrl string, subscribeForm url.Values) (resp *http.Response, err error) {
	return http.PostForm(subscribeUrl, subscribeForm)
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
		log.Printf("Removing old handler [%v]", h.Id)
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
