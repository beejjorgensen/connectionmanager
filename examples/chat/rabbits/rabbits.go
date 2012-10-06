// Hammer away at the chat server with fake connections
package main

import (
	"code.google.com/p/go-uuid/uuid"
	"launchpad.net/gnuflag"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"runtime"
	"time"
	"os"
)

const msgurl = "http://127.0.0.1:8080/cmd"
const pollurl = "http://127.0.0.1:8080/poll"

var connectionCount int
var minDelay int
var maxDelay int

// Goroutine to run a long poll thread for a robot
func robotPoller(id string) {
	var payload interface{}

	transport := new(http.Transport)
	transport.DisableKeepAlives = true
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
	transport.DisableKeepAlives = true
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
		duration := rand.Int31n(int32(maxDelay-minDelay)) + int32(minDelay)
		time.Sleep(time.Duration(duration) * time.Millisecond)

		transport.CloseIdleConnections()
	}
}

// Usage
func usage() {
	fmt.Fprintf(os.Stderr, "usage %s [options]\n", os.Args[0])
	gnuflag.PrintDefaults()
}

// Main
func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	/*
		connectionCount = 200
		minDelay = 5000 // ms
		maxDelay = 20000
	connectionCount = 20 
	minDelay = 1500 // ms
	maxDelay = 4000
	*/

	gnuflag.Usage = usage

	gnuflag.IntVar(&connectionCount, "c", 20, "number of simultaneous bots to run")
	gnuflag.IntVar(&minDelay, "n", 1500, "minimum delay time between chats (ms)")
	gnuflag.IntVar(&maxDelay, "x", 4000, "maximum delay time between chats (ms)")

	gnuflag.Parse(true)

	if connectionCount < 1 || minDelay < 0 || maxDelay < 0 {
		usage()
		os.Exit(1)
	}

	if minDelay > maxDelay {
		t := minDelay
		minDelay = maxDelay
		maxDelay = t
	}

	for i := 0; i < connectionCount; i++ {
		go robot()
	}

	fmt.Printf("Hit return to quit")
	fmt.Scanln()
}
