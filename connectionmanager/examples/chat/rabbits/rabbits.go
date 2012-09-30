// Hammer away at the chat server with fake connections
package main

import (
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"time"
)

const msgurl = "http://127.0.0.1:8080/cmd"
const pollurl = "http://127.0.0.1:8080/poll"

var connectionCount int
var minDelay int32
var maxDelay int32

// Goroutine to run a long poll thread for a robot
func robotPoller(id string) {
	var payload interface{}

	transport := new(http.Transport)
	client := &http.Client{Transport: transport}

	for {
		// send in a poll request and wait for response
		resp, err := client.PostForm(pollurl,
			url.Values{"id": {id}})

		if err != nil {
			log.Printf("poller post error: %v", err)
			return
		}

		//log.Printf("************** %v %v", resp, err)
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			log.Printf("http poll request error: %v", err)
			return
		}

		// extract payload
		err = json.Unmarshal(body, &payload)

		if err != nil {
			log.Printf("json poll unmarshal error: %v", err)
			log.Printf("json poll data: %s", string(body))
			return
		}

		outerObj := payload.([]interface{})

		for _, v := range outerObj {
			message := v.(map[string]interface{})

			respType := message["type"].(string)

			// check for login errors
			if respType == "status" {
				status := message["status"].(string)
				if status == "error" {
					message := message["message"].(string)
					log.Printf("chat server pollrequest error: %s", message)
					return
				}
			}

			// parse message
			switch respType {
			case "message":
				//log.Printf("%s: message: %s: %s", id, message["username"].(string), message["message"].(string))

			case "newuser":
				//log.Printf("%s: new user: %s", id, message["username"].(string))

			default:
				log.Printf("%s: unknown type: %s", id, respType)
			}
		}

		transport.CloseIdleConnections()
	}
}

// Goroutine to run a robot
func robot() {
	var payload interface{}

	transport := new(http.Transport)
	client := &http.Client{Transport: transport}

	id := uuid.New()

	// login
	resp, err := client.PostForm(msgurl,
		url.Values{"command": {"login"}, "id": {id}})

	if err != nil {
		log.Printf("http login error: %v", err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		log.Printf("http login error: %v", err)
		return
	}

	// extract payload
	err = json.Unmarshal(body, &payload)

	if err != nil {
		log.Printf("json unmarshal error: %v", err)
		log.Printf("json data: %s", string(body))
		return
	}

	outerObj := payload.(map[string]interface{})
	respType := outerObj["type"].(string)

	// check for login errors
	if respType == "status" {
		status := outerObj["status"].(string)
		if status == "error" {
			message := outerObj["message"].(string)
			log.Printf("chat server login error: %s", message)
			return
		}
	}

	// get additional user data
	//publicId := outerObj["publicid"].(string)
	//userName := outerObj["username"].(string)

	// run the poller
	go robotPoller(id)

	msgCount := 0

	// main loop
	for {
		// send chat
		resp, err = client.PostForm(msgurl,
			url.Values{
				"command": {"broadcast"},
				"id":      {id},
				"message": {fmt.Sprintf("message %d", msgCount)},
			})

		msgCount++

		if err != nil {
			log.Printf("http broadcast error: %v", err)
			return
		}

		body, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		// check for broadcast errors
		if respType == "status" {
			status := outerObj["status"].(string)
			if status == "error" {
				message := outerObj["message"].(string)
				log.Printf("chat server broadcast error: %s", message)
				return
			}
		}

		// sleep a bit
		time.Sleep(time.Duration((rand.Int31n(maxDelay-minDelay) + minDelay)) * time.Millisecond)

		transport.CloseIdleConnections()
	}
}

// Main
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	/*
		connectionCount = 200
		minDelay = 5000 // ms
		maxDelay = 20000
	*/
	connectionCount = 200
	minDelay = 500 // ms
	maxDelay = 1500

	for i := 0; i < connectionCount; i++ {
		go robot()
	}

	fmt.Printf("Hit return to quit")
	fmt.Scanln()
}
