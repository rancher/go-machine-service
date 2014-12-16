package test_utils

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
)

var subscriberChannels []chan string

func publishHandler(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "A response.")
}

func pushEventHandler(w http.ResponseWriter, req *http.Request) {
	bod, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	body := string(bod[:len(bod)])
	pushToSubscribers(body)
}

func subscribeHandler(w http.ResponseWriter, req *http.Request) {
	resultChan := make(chan string)
	subscriberChannels = append(subscriberChannels, resultChan)
	writeEventToSubscriber(w, resultChan)
}

func pushToSubscribers(message string) {
	if len(subscriberChannels) > 0 {
		for i := range subscriberChannels {
			log.Printf("sending events: %s", message)
			subscriberChannels[i] <- message
		}
	}
}

func writeEventToSubscriber(w http.ResponseWriter, c chan string) {
	for {
		io.WriteString(w, <-c+"\r\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func readyHandler(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "Ready")
}

func InitializeServer(port string, ready chan string) (err error) {
	http.HandleFunc("/subscribe", subscribeHandler)
	http.HandleFunc("/publish", publishHandler)
	http.HandleFunc("/pushEvent", pushEventHandler)
	http.HandleFunc("/ready", readyHandler)
	go http.ListenAndServe(":"+port, nil)

	readyUrl := "http://localhost:" + port + "/pushEvent"
	for {
		resp, err := http.Post(readyUrl, "application/json", nil)
		// TODO This was added when I was debuggin. Might not need it now.
		if err == nil {
			log.Println(resp.Status)
			break
		} else {
			log.Fatal(err)
		}
	}

	ready <- "Ready!"
	return nil
}
