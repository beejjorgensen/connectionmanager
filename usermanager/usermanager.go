//Package usermanager provides communication between users who are
//connected in different goroutines.
package usermanager

import (
	"code.google.com/p/go-uuid/uuid"
	"container/list"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
)

// TODO: make sure JSON can encode a struct with embedded maps, private
// entities in struct, then move a lot of stuff out of the map into the
// structs

// TODO: generalize--is this a pub/sub model?

// TODO: allow sending arbitrary messages with no sender?
// TODO: unicast request
// TODO: multicast request (register multicast groups?)
// TODO: rework main thread to use functions
// TODO: have general storage for the User so we don't have setUserName, setAge, etc.

// Message type identifiers
const (
	StopRequest       = "stop"
	StopResponse      = "stopresponse"
	LoginRequest      = "login"
	LoginResponse     = "loginresponse"
	NewUserRequest    = "newuserrequest"
	NewUserResponse   = "newuserresponse"
	SetAttrRequest    = "setattrrequest"
	SetAttrResponse   = "setattrresponse"
	GetAttrRequest    = "getattrrequest"
	GetAttrResponse   = "getattrresponse"
	BroadcastRequest  = "broadcastrequest"
	BroadcastResponse = "broadcastresponse"
	PollRequest       = "pollrequest"
	PollResponse      = "pollresponse"

	Broadcast = "broadcast"
)

// This dictates how many reentrant calls to SendRequest() can be made
// from a single thread without deadlocking (as if the thread made a
// second call to SendRequest() while servicing a side effect of a first
// call). I would not expect more than 3 in normal usage.
const requestChannelSize = 50

// Information about a particular user
type User struct {
	// channel for receiving messages for this user
	pollChannel chan string // for sending data to the poll connection
	polling     bool        // true if the user is polling 
	messages    *list.List  // undelivered messages

	attr map[string]string // general attributes

	id    string // secret
	pubId string // public
}

// Class for holding requests to the UserManager
type Request map[string]interface{}

// Manages users
type UserManager struct {
	user           map[string]*User  // map id to User
	idMap          map[string]string // map pubId to id
	requestChannel chan *Request
	active         bool
}

// Check if a user is polling, and send responses
func (u *User) pollCheck() {
	l := u.messages.Len()
	if u.polling && l > 0 {
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

	}
}

// Allocate and initialize a new user
func newUser(userId string) *User {
	user := &User{
		pollChannel: nil, // will be set when the user starts polling
		polling:     false,
		messages:    list.New(),
		id:          userId,
		pubId:       uuid.New(),
		attr:        make(map[string]string),
	}

	return user
}

// remove a user from tracking
func (um *UserManager) removeUser(user *User) {
	// TODO
}

// Start or stop a user manager service
func (um *UserManager) SetActive(active bool) {
	if active {
		if !um.active {
			go runUserManager(um)
			um.active = true
		}
	} else {
		if um.active {
			um.SendRequest(&Request{
				"type": StopRequest,
			})
		}
	}
}

// Create a new UserManager
func New() *UserManager {
	userManager := &UserManager{
		user:           make(map[string]*User),
		requestChannel: make(chan *Request, requestChannelSize),
	}

	return userManager
}

// Returns exportable user data
func (um *UserManager) buildUserListResponse() []map[string]interface{} {
	// make an array of maps
	r := make([]map[string]interface{}, len(um.user))

	i := 0
	for _, u := range um.user {
		r[i] = map[string]interface{}{
			"id":       u.pubId,
			"userdata": u.attr,
		}
		i++
	}

	return r
}

// Sends a Request and receives a Request
//
// to be called from other threads
func (um *UserManager) SendRequest(r *Request) *Request {
	rchan := make(chan *Request)

	(*r)["_return"] = rchan

	log.Printf("SendRequest: sending %s\n", *r)
	um.requestChannel <- r
	log.Printf("SendRequest: receiving\n")
	resp := <-rchan
	log.Printf("SendRequest: received %s\n", *resp)

	delete(*r, "_return")

	return resp
}

// Broadcasts a response to all users
func (um *UserManager) broadcast(r *Request) {
	for _, u := range um.user {
		// buffer to all users
		u.messages.PushBack(r)

		// send if polling
		u.pollCheck()
	}
}

// Manages users (runs as a goroutine)
func runUserManager(userManager *UserManager) {
	for {
		var u *User
		var present bool

		log.Println("UserManager: waiting for request")

		request := *(<-userManager.requestChannel)
		requestType := request["type"]

		log.Printf("UserManager: got request: %s\n", request)

		// if request is for polling data, send it
		switch requestType {

		case SetAttrRequest:
			userId := request["id"].(string)
			attrName := request["attr"].(string)
			value := request["value"].(string)

			u = userManager.user[userId]
			u.attr[attrName] = value

			rchan := request["_return"].(chan *Request)
			log.Println("UserManager: sending setattr response")
			rchan <- &Request{
				"type":   SetAttrResponse,
				"status": "ok",
			}
			log.Println("UserManager: sent setattr response")

		case GetAttrRequest:
			userId := request["id"].(string)
			attrName := request["attr"].(string)

			u = userManager.user[userId]

			value, ok := u.attr[attrName]

			rchan := request["_return"].(chan *Request)
			log.Println("UserManager: sending getattr response")
			rchan <- &Request{
				"type":   GetAttrResponse,
				"value":  value,
				"status": strconv.FormatBool(ok),
			}
			log.Println("UserManager: sent getattr response")

		case LoginRequest:
			userId := request["id"].(string)
			userAttrs := request["userdata"].(map[string]string)

			// make a new user if we don't have it
			if u, present = userManager.user[userId]; !present {
				u = newUser(userId)

				log.Printf("UserManager: %s: new user\n", userId)
			}

			// copy user attributes
			if userAttrs != nil {
				for k, v := range userAttrs {
					log.Printf("attr: %s=%s, %s\n", k, v, u)
					u.attr[k] = v
				}
			}

			// broadcast to everyone else
			userManager.broadcast(&Request{
				"type":     NewUserResponse,
				"pubId":    u.pubId,
				"userdata": u.attr,
			})

			// build map of logged-in user data for reply
			userList := userManager.buildUserListResponse()

			// add to the list
			userManager.user[userId] = u

			// send response
			rchan := request["_return"].(chan *Request)
			log.Println("UserManager: sending login response")
			rchan <- &Request{
				"type":     LoginResponse,
				"id":       userId,
				"pubId":    u.pubId,
				"userdata": u.attr,
				"userlist": userList,
				"status":   "ok",
			}
			log.Println("UserManager: sent login response")

		case StopRequest:
			rchan := request["_return"].(chan *Request)
			log.Println("UserManager: sending stop response")
			rchan <- &Request{
				"type":   StopResponse,
				"status": "ok",
			}
			log.Println("UserManager: sent stop response")
			return // exit from the goroutine

		case PollRequest:
			userId := request["id"].(string)
			u := userManager.user[userId]
			rchan := request["_return"].(chan *Request)

			if u == nil {
				log.Printf("UserManager: unknown user ID for PollRequest: %s\n", userId)
				rchan <- &Request{
					"type":    PollResponse,
					"message": "unknown user ID",
					"status":  "error",
				}
				break
			}

			// If the user is already polling, it means they've opened a
			// new polling connection. The old one needs to be shut down
			// before we can continue
			if u.polling {
				close(u.pollChannel)
			}

			// mark user as polling
			u.polling = true
			u.pollChannel = make(chan string)

			log.Println("UserManager: sending pollrequest response")
			rchan <- &Request{
				"type":     PollResponse,
				"userchan": u.pollChannel,
				"status":   "ok",
			}
			log.Println("UserManager: sent pollrequest response")

			// push if we already have something
			u.pollCheck()

		case BroadcastRequest:
			userId := request["id"].(string)
			userData := request["userdata"].(map[string]string)
			u = userManager.user[userId]

			response := &Request{
				"type":     Broadcast,
				"fromId":   u.pubId,
				"userdata": userData,
			}

			// buffer messages and push to waiting users
			userManager.broadcast(response)

			rchan := request["_return"].(chan *Request)
			log.Println("UserManager: sending broadcast response")
			rchan <- &Request{
				"type":   BroadcastResponse,
				"status": "ok",
			}
			log.Println("UserManager: sent broadcast response")

		default:
			panic(fmt.Sprintf("Unknown request: \"%s\"", requestType))
		}

		log.Println("UserManager: finished servicing request")
	}
}
