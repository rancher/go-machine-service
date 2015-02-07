package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/rancherio/go-machine-service/locks"
	"github.com/rancherio/go-rancher/client"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

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
	registerUrl         string
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
	registerForm := url.Values{}
	subscribeForm := url.Values{}
	// TODO Understand need/function of UUID
	registerForm.Set("uuid", router.name)
	registerForm.Set("name", router.name)
	registerForm.Set("priority", strconv.Itoa(router.priority))

	eventHandlerSuffix := ";handler=" + router.name
	handlers := map[string]EventHandler{}

	if pingHandler, ok := router.eventHandlers["ping"]; ok {
		// Ping doesnt need registered in the POST and
		// ping events don't have the handler suffix. If we
		// start handling other non-suffix events,
		// we might consider improving this.
		handlers["ping"] = pingHandler
	}

	for event, handler := range router.eventHandlers {
		registerForm.Add("processNames", event)
		fullEventKey := event + eventHandlerSuffix
		subscribeForm.Add("eventNames", fullEventKey)
		handlers[fullEventKey] = handler
	}

	regResponse, err := http.PostForm(router.registerUrl, registerForm)
	if err != nil {
		return err
	}
	defer regResponse.Body.Close()

	if ready != nil {
		ready <- true
	}

	// TODO Harden. Add reconnect logic.
	eventStream, err := http.PostForm(router.subscribeUrl, subscribeForm)
	if err != nil {
		return err
	}
	log.Println("Connection established.")
	router.eventStreamResponse = eventStream
	defer eventStream.Body.Close()

	scanner := bufio.NewScanner(eventStream.Body)
	for scanner.Scan() {
		line := scanner.Bytes()

		// TODO Ensure this wont break eventing paradigm
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
	// TODO Revisit. Not sure if I need/want this
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
	// TODO Is the right way to defer body closing right after getting response, before checking error?
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
		registerUrl:   apiUrl + "/externalhandlers",
		subscribeUrl:  apiUrl + "/subscribe",
		replyUrl:      apiUrl + "/publish",
		eventHandlers: eventHandlers,
		workerCount:   workerCount,
	}, nil
}

func newWorker(replyUrl string) *Worker {
	return &Worker{}
}
