// A long-polling HTTP chat server
package main

// TODO:
//
// get WEBROOT on command line or environment
// handle users who leave
// handle idle users
// response JSON builder should dynamically build the JSON
// hide private ID from snoopers

import (
	"bufio"
	"code.google.com/p/go-uuid/uuid"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"time"
	"usermanager"
)

type user struct {
	name string
	pubId string // public identifier
}

// Manages user structs
type UserManager struct {
	// Maps id to user
	user           map[string]*user

	// Maps pubId to id
	idMap          map[string]string

	// For making anonymous user names
	nextGuestNumber int
}

// Add a new user
//
// Returns pointer to user
func (um *UserManager) addUser(id string, name string) *user {
	u, ok := um.user[id]

	if !ok {
		u = new(User)
		u.name = name
		u.id = id
		u.pubId = uuid.New() // v4 UUID

		um.idMap[u.pubId] = id
	}

	return u
}


// Return a user by ID
func (um *UserManager) getUserById(id string) *user, bool {
	return um.user[id]
}

// Return a user by PubID
func (um *UserManager) getUserByPubId(pid string) *user, bool {
	id, ok := um.idMap[pid]

	if !ok {
		return nil, ok
	}

	return um.getUserById(id)
}


		/*

Old polling response code
		messageIndex := make([]*Request, l)

		// can't json.Marshal a List, so we put references in a slice:
		count := 0
		for e := u.messages.Front(); e != nil; e = e.Next() {
			msg := e.Value.(*Request)
			messageIndex[count] = msg
			count++
		}

		messages, err := json.Marshal(messageIndex)

		if err != nil {
			panic(err)
		}

		// ditch sent messages
		u.messages.Init()

		log.Printf("UserManager: pollCheck: sending to %s: %s\n", u.id, string(messages))
		u.pollChannel <- string(messages)
		log.Printf("UserManager: pollCheck: sending to %s: complete\n", u.id)

		// unmark users as polling
		u.polling = false

*/

const WEBROOT = "/home/beej/src/golang/longpoll/webroot"

// Web file open error
type WebFileOpenError struct {
	msg string
}

// WebFileOpenError implementation
func (w *WebFileOpenError) Error() string {
	return w.msg
}

// Handler for user command requests (implements http.Handler)
type CommandHandler struct {
	userManager *usermanager.UserManager
}

// Handler for long-poll requests (implements http.Handler)
type LongPollHandler struct {
	userManager *usermanager.UserManager
}

// Helper function to get JSON strings
func toJSONStr(data *usermanager.Request) []byte {
	s, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}

	return s
}

// Service user command HTTP requests
func (h *CommandHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	var userName string
	var resp *usermanager.Request

	// get passed parameters
	id := rq.FormValue("id")
	com := rq.FormValue("command")

	log.Printf("Chat: serving command %s: %s\n", com, id)

	switch com {
	case "login":
		// extract username
		userName = rq.FormValue("username")

		// autogen anon names if unspecified?
		if userName == "" {
			userName = fmt.Sprintf("Guest%d", nextGuestNumber)
			nextGuestNumber++
		}

		resp = h.userManager.SendRequest(&usermanager.Request{
			"type": usermanager.LoginRequest,
			"id":   id,
			"userdata": map[string]string{
				"username": userName,
			},
		})

		rw.Write(toJSONStr(resp))

	case "broadcast":
		// extract message
		msg := rq.FormValue("message")

		// from username
		resp := h.userManager.SendRequest(&usermanager.Request{
			"type": usermanager.GetAttrRequest,
			"id":   id,
			"attr": "username",
		})

		// broadcast to all users
		resp = h.userManager.SendRequest(&usermanager.Request{
			"type": usermanager.BroadcastRequest,
			"id":   id,
			"userdata": map[string]string{
				"username": (*resp)["value"].(string), // from get username, above
				"msg":      msg,
			},
		})

		rw.Write(toJSONStr(resp))

	case "setusername":
		userName = rq.FormValue("username")

		resp := h.userManager.SendRequest(&usermanager.Request{
			"type":  usermanager.SetAttrRequest,
			"id":    id,
			"attr":  "username",
			"value": userName,
		})

		rw.Write(toJSONStr(resp))
	}
}

// Service long-poll HTTP requests
func (h *LongPollHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	// get ID
	id := rq.FormValue("id")

	log.Printf("Chat: beginning long poll: %s\n", id)

	// request messages from UserManager
	resp := h.userManager.SendRequest(&usermanager.Request{
		"type": usermanager.PollRequest,
		"id":   id,
	})

	// check for errors
	if (*resp)["status"] == "error" {
		log.Printf("Chat: long poll request error: %s\n", (*resp)["message"])
		fmt.Printf(`{"type":"error", "message":"%s"}`, (*resp)["message"])
		return
	}

	// wait for messages
	userChan := (*resp)["userchan"].(chan string)
	log.Printf("Chat: long poll waiting on channel: %s\n", userChan)

	msgJSON, ok := <-userChan

	if ok {
		log.Printf("Chat: long poll response received: %s\n", id)

		// send messages in response
		fmt.Fprintf(rw, msgJSON)

		log.Printf("Chat: completed long poll: %s\n", id)
	} else {
		log.Printf("Chat: long poll channel closed\n")
	}

}

// Try to open a file. Returns the file or nil.
func getOpenFile(filePath string) (file *os.File, rerr error) {
	var fi os.FileInfo
	var err error

	fi, err = os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	if fi.IsDir() {
		return nil, &WebFileOpenError{"file is directory"}
	}

	file, err = os.Open(filePath)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// General file handler
func fileHandler(rw http.ResponseWriter, r *http.Request) {
	var file *os.File
	var err error
	var usedFilePath *string

	// Attempt to open the asked-for file, or index.html
	filePath := fmt.Sprintf("%s%s", WEBROOT, r.URL.RequestURI())
	usedFilePath = &filePath

	file, err = getOpenFile(filePath)
	if err != nil {
		indexPath := fmt.Sprintf("%s%cindex.html", filePath, os.PathSeparator)
		usedFilePath = &indexPath
		file, err = getOpenFile(indexPath)
		if err != nil {
			log.Printf("error opening %s\n", filePath)
			rw.WriteHeader(http.StatusNotFound)
			io.WriteString(rw, "404 Not Found")
			return
		}
	}

	defer file.Close()

	// Set the MIME type
	mimeType := mime.TypeByExtension(path.Ext(*usedFilePath))

	if mimeType == "" {
		mimeType = "text/plain"
	}

	rw.Header().Set("Content-Type", mimeType)

	log.Printf("serving %s\n", filePath)

	// Serve the file
	reader := bufio.NewReader(file)
	buf := make([]byte, 1024)

	for {
		n, err := reader.Read(buf)

		if err != nil && err != io.EOF {
			panic(err)
		}
		if n == 0 {
			break
		}

		n2, err2 := rw.Write(buf[:n])
		if err2 != nil || n2 != n {
			panic("file write error")
		}
	}
}

// Sets up the handlers and runs the HTTP server (run as a goroutine)
func runServer(userManager *usermanager.UserManager) {
	longPollHandler := new(LongPollHandler)
	commandHandler := new(CommandHandler)

	longPollHandler.userManager = userManager
	commandHandler.userManager = userManager

	s := &http.Server{
		Addr:           ":8080",
		Handler:        nil,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   /*120*/ 5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	http.Handle("/poll", longPollHandler)
	http.Handle("/cmd", commandHandler)
	http.HandleFunc("/", fileHandler)

	log.Fatal(s.ListenAndServe())
}

// main
func main() {
	userManager := usermanager.New()
	userManager.SetActive(true)

	log.Println("Running server")
	go runServer(userManager)

	// console
	fmt.Scanln()
}
