// A long-polling HTTP chat server
package main

// TODO:
//
// add some kind of console thing?
// get WEBROOT on command line or environment
// handle users who leave
// handle idle users

import (
	"code.google.com/p/go-uuid/uuid"
	"github.com/beej71/connectionmanager"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"
)

const WEBROOT = "/home/beej/src/go/libs/src/github.com/beej71/connectionmanager/examples/chat/webroot"

type response map[string]interface{}

type User struct {
	name  string
	id    string
	pubId string // public identifier
}

// Manages user structs
type UserManager struct {
	// Maps id to user
	user map[string]*User

	// Maps pubId to id
	idMap map[string]string

	// For making anonymous user names
	nextGuestNumber int
}

// Construct a new UserManager
func newUserManager() *UserManager {
	userManager := &UserManager{
		user: make(map[string]*User),
		idMap: make(map[string]string),
		nextGuestNumber: 1,
	}

	return userManager
}

// Autogenerate a unique printable username
func (um *UserManager) nextGuestName() string {
	name := fmt.Sprintf("Guest%d", um.nextGuestNumber)
	um.nextGuestNumber++

	return name
}

// Add a new user
//
// Returns pointer to user
func (um *UserManager) addUser(id string, name string) *User {
	u, ok := um.user[id]

	if name == "" {
		name = um.nextGuestName()
	}

	if !ok {
		u = new(User)
		u.name = name
		u.id = id
		u.pubId = uuid.New() // v4 UUID

		um.idMap[u.pubId] = id

		um.user[u.id] = u
	}

	return u
}

// Return a user by ID
func (um *UserManager) getUserById(id string) (*User, bool) {
	u, ok := um.user[id]
	return u, ok
}

// Return a user by PubID
func (um *UserManager) getUserByPubId(pid string) (*User, bool) {
	id, ok := um.idMap[pid]

	if !ok {
		return nil, ok
	}

	return um.getUserById(id)
}

// Handler for user command requests (implements http.Handler)
type CommandHandler struct {
	userManager       *UserManager
	connectionManager *connectionmanager.ConnectionManager
}

// Handler for long-poll requests (implements http.Handler)
type LongPollHandler struct {
	userManager       *UserManager
	connectionManager *connectionmanager.ConnectionManager
}

// Helper function to make a status response
func makeStatusResponse(status string, message string) *response {
	return &response{
		"type":    "status",
		"status":  status,
		"message": message,
	}
}

// Helper function to log errors in response writes
func writeReponse(rw http.ResponseWriter, data *[]byte) {
	n, err := rw.Write(*data)
	if err != nil {
		log.Printf("error writing command response: %v", err)
		return
	}

	l := len(*data)
	if n != l {
		log.Printf("command response short write: %d bytes (out of %d)", n, l)
	}
}

// Service user command HTTP requests
func (h *CommandHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	var userName string
	var resp *connectionmanager.Message
	var jresp []byte

	// get passed parameters
	id := rq.FormValue("id")
	com := rq.FormValue("command")

	//log.Printf("Chat: serving command %s: %s\n", com, id)

	switch com {
	case "login":
		// extract username
		userName = rq.FormValue("username")

		resp = h.connectionManager.SendMessage(&connectionmanager.Message{
			Type: connectionmanager.ConnectRequest,
			Id:   id,
		})

		// if ok, record in our user list
		if resp.Err == nil {
			user := h.userManager.addUser(id, userName)
			jresp, _ = json.Marshal(response{
				"type":     "loginresponse",
				"id":       id,
				"publicid": user.pubId,
				"username": user.name,
			})

			// notify all other users of the new user
			// send the broadcast request
			h.connectionManager.SendMessage(&connectionmanager.Message{
				Type: connectionmanager.BroadcastRequest,
				Id:   id,
				Payload: &connectionmanager.MessagePayload{
					"type":     "newuser",
					"username": user.name,
					"publicid": user.pubId,
				},
			})

			//jresp, _ = json.Marshal(*makeStatusResponse("ok", ""))

		} else {
			jresp, _ = json.Marshal(*makeStatusResponse("error", resp.Err.Error()))
		}

		writeReponse(rw, &jresp)

	case "broadcast":
		// extract message
		msg := rq.FormValue("message")

		user, ok := h.userManager.getUserById(id)

		if ok {
			// send the broadcast request
			h.connectionManager.SendMessage(&connectionmanager.Message{
				Type: connectionmanager.BroadcastRequest,
				Id:   id,
				Payload: &connectionmanager.MessagePayload{
					"type":     "message",
					"username": user.name,
					"publicid": user.pubId,
					"message":  msg,
				},
			})
			jresp, _ = json.Marshal(*makeStatusResponse("ok", ""))

		} else {
			jresp, _ = json.Marshal(*makeStatusResponse("error", fmt.Sprintf("user not found: %s", id)))
		}

		writeReponse(rw, &jresp)

	case "setusername":
		userName = rq.FormValue("username")

		user, ok := h.userManager.getUserById(id)

		if ok {
			// broadcast that we're changing our username
			h.connectionManager.SendMessage(&connectionmanager.Message{
				Type: connectionmanager.BroadcastRequest,
				Id:   id,
				Payload: &connectionmanager.MessagePayload{
					"type":        "changeusername",
					"oldusername": user.name,
					"newusername": user.name,
					"publicid":    user.pubId,
				},
			})

			// change username
			user.name = userName

			jresp, _ = json.Marshal(*makeStatusResponse("ok", ""))

		} else {
			jresp, _ = json.Marshal(*makeStatusResponse("error", fmt.Sprintf("user not found: %s", id)))
		}

		writeReponse(rw, &jresp)
	}
}

// Service long-poll HTTP requests
func (h *LongPollHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	var jresp []byte

	rw.Header().Set("Content-Type", "application/json")

	// get ID
	id := rq.FormValue("id")

	//log.Printf("Chat: beginning long poll: %s\n", id)

	// request messages from ConnectionManager
	resp := h.connectionManager.SendMessage(&connectionmanager.Message{
		Type: connectionmanager.PollRequest,
		Id:   id,
	})

	// check for errors
	if resp.Err != nil {
		log.Printf("Chat: long poll request error: %s", resp.Err.Error())

		jresp, _ := json.Marshal(*makeStatusResponse("error", resp.Err.Error()))
		writeReponse(rw, &jresp)

		return
	}

	// wait for messages
	//log.Printf("Chat: long poll waiting on channel: %s\n", resp.RChan)

	// response channel in resp
	pollresp, ok := <-resp.PollChan

	if ok {
		messages := make([]*connectionmanager.MessagePayload, len(*pollresp))

		for i, v := range *pollresp {
			messages[i] = v.Payload
		}

		//log.Printf("Chat: long poll response received: %s\n", id)

		// send messages in response
		jresp, _ = json.Marshal(messages)

		//log.Printf("Chat: completed long poll: %s\n", id)
	} else {
		//log.Printf("Chat: long poll channel closed\n")

		// the remote side has probably closed at this point, but let's
		// send a response anyway
		jresp, _ = json.Marshal(*makeStatusResponse("error", "long poll canceled"))
	}

	//log.Printf(">>>>> %s", string(jresp))
	writeReponse(rw, &jresp)
}

// General file server
func fileHandler(rw http.ResponseWriter, r *http.Request) {
	filePath := fmt.Sprintf("%s%s", WEBROOT, r.URL.RequestURI())
	http.ServeFile(rw, r, filePath)
}

// Sets up the handlers and runs the HTTP server (run as a goroutine)
func runWebServer(connectionManager *connectionmanager.ConnectionManager,
	userManager *UserManager) {

	longPollHandler := &LongPollHandler{
		connectionManager: connectionManager,
		userManager: userManager,
	}

	commandHandler := &CommandHandler{
		connectionManager: connectionManager,
		userManager: userManager,
	}

	s := &http.Server{
		Addr:           ":8080",
		Handler:        nil,
		ReadTimeout:    120 * time.Second,
		WriteTimeout:   2 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	http.Handle("/poll", longPollHandler)
	http.Handle("/cmd", commandHandler)
	http.HandleFunc("/", fileHandler)

	log.Fatal(s.ListenAndServe())
}

// main
func main() {
	log.Printf("num cpus: %v", runtime.NumCPU())
	runtime.GOMAXPROCS(runtime.NumCPU())

	userManager := newUserManager()
	connectionManager := connectionmanager.New()
	connectionManager.SetActive(true)

	log.Println("Running server")

	go runWebServer(connectionManager, userManager)

	// console
	fmt.Scanln()
}