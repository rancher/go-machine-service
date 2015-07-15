package test_utils

import (
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"
)

var subscriberChannels []chan string

func ResetTestServer() {
	for _, channel := range subscriberChannels {
		close(channel)
	}
	subscriberChannels = subscriberChannels[:0]
}

func publishHandler(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "A response.")
}

func pushEventHandler(w http.ResponseWriter, req *http.Request) {
	bod, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	body := string(bod[:])
	pushToSubscribers(body)
}

func subscribeHandler(w http.ResponseWriter, req *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	ws, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection.", 500)
		return
	}

	resultChan := make(chan string)
	subscriberChannels = append(subscriberChannels, resultChan)
	writeEventToSubscriber(ws, resultChan)
}

func pushToSubscribers(message string) {
	if len(subscriberChannels) > 0 {
		for _, channel := range subscriberChannels {
			log.Printf("sending events: %s", message)
			channel <- message
		}
	}
}

func writeEventToSubscriber(ws *websocket.Conn, c chan string) {
	for {
		event := <-c
		err := ws.WriteMessage(websocket.TextMessage, []byte(event))
		if err != nil {
			log.WithFields(log.Fields{"error": err, "msg": event}).Fatal("Could not write message.")
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
