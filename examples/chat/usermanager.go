// User Manager code for chat.go
//
// visible from chat.go
package main

import (
	"code.google.com/p/go-uuid/uuid"
	"errors"
	"fmt"
)

const (
	commandStop           = 0
	commandGetUserByID    = 1
	commandGetUserByPubID = 2
	commandAddUser        = 3
	commandRemoveUser     = 4
)

type userManagerCommand struct {
	rchan       chan *userManagerCommandResponse
	commandType int
	payload     interface{}
}

type userManagerCommandResponse struct {
	payload interface{}
	err     error
}

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

	requestChan chan *userManagerCommand
}

// Autogenerate a unique printable username
func (um *UserManager) nextGuestName() string {
	name := fmt.Sprintf("Guest%d", um.nextGuestNumber)
	um.nextGuestNumber++

	return name
}

// Add a new user
func (um *UserManager) internalAddUser(command *userManagerCommand) *userManagerCommandResponse {
	payload := command.payload.([]string)
	id := payload[0]
	name := payload[1]

	u, ok := um.user[id]

	if ok {
		return &userManagerCommandResponse{
			err: errors.New(fmt.Sprintf("user already exists: %s", id)),
		}
	}

	if name == "" {
		name = um.nextGuestName()
	}

	u = &User{
		name:  name,
		id:    id,
		pubId: uuid.New(), // v4 UUID
	}

	um.idMap[u.pubId] = id
	um.user[u.id] = u

	return &userManagerCommandResponse{payload: *u}
}

// Return a user by ID
func (um *UserManager) internalGetUserById(command *userManagerCommand) *userManagerCommandResponse {
	var resp *userManagerCommandResponse

	id := command.payload.(string)

	u, ok := um.user[id]

	if ok {
		resp = &userManagerCommandResponse{payload: *u}
	} else {
		resp = &userManagerCommandResponse{
			err: errors.New(fmt.Sprintf("unknown user ID: %s", id)),
		}
	}

	return resp
}

// Return a user by PubID
func (um *UserManager) internalGetUserByPubId(command *userManagerCommand) *userManagerCommandResponse {
	var resp *userManagerCommandResponse

	pid := command.payload.(string)

	id, ok := um.idMap[pid]
	if !ok {
		return &userManagerCommandResponse{
			err: errors.New(fmt.Sprintf("unknown user pubID: %s", id)),
		}
	}

	u, ok := um.user[id]

	if ok {
		resp = &userManagerCommandResponse{payload: *u}
	} else {
		resp = &userManagerCommandResponse{
			err: errors.New(fmt.Sprintf("unknown user ID %s via pubID %s", id, pid)),
		}
	}

	return resp
}

// Stop the server
func (um *UserManager) internalStop(command *userManagerCommand) *userManagerCommandResponse {
	return &userManagerCommandResponse{}
}

func (um *UserManager) requestHandler() {
	for {
		command := <-um.requestChan
		switch command.commandType {
		case commandStop:
			command.rchan <- um.internalStop(command)
			return

		case commandGetUserByID:
			command.rchan <- um.internalGetUserById(command)

		case commandGetUserByPubID:
			command.rchan <- um.internalGetUserByPubId(command)

		case commandAddUser:
			command.rchan <- um.internalAddUser(command)
		}
	}
}

// Helper function to return a *User,error pair
func getUserOrError(umr *userManagerCommandResponse) (*User, error) {
	if umr.err != nil {
		return nil, umr.err
	}

	user := umr.payload.(User)
	return &user, nil
}

// Construct a new UserManager
func NewUserManager() *UserManager {
	userManager := &UserManager{
		user:            make(map[string]*User),
		idMap:           make(map[string]string),
		nextGuestNumber: 1,
		requestChan:     make(chan *userManagerCommand),
	}

	return userManager
}

// Start the user manager main loop
func (um *UserManager) Start() {
	go um.requestHandler()
}

// Stop the UserManager
func (um *UserManager) Stop() error {
	rchan := make(chan *userManagerCommandResponse)
	um.requestChan <- &userManagerCommand{
		rchan:       rchan,
		commandType: commandStop,
	}

	umr := <-rchan

	return umr.err
}

// Get a user by ID
func (um *UserManager) GetUserByID(id string) (*User, error) {
	rchan := make(chan *userManagerCommandResponse)
	um.requestChan <- &userManagerCommand{
		rchan:       rchan,
		commandType: commandGetUserByID,
		payload:     id,
	}

	umr := <-rchan

	return getUserOrError(umr)
}

// Get a user by public ID
func (um *UserManager) GetUserByPubID(id string) (*User, error) {
	rchan := make(chan *userManagerCommandResponse)
	um.requestChan <- &userManagerCommand{
		rchan:       rchan,
		commandType: commandGetUserByPubID,
		payload:     id,
	}

	umr := <-rchan

	return getUserOrError(umr)
}

// Add a new user
func (um *UserManager) AddUser(id string, name string) (*User, error) {
	rchan := make(chan *userManagerCommandResponse)

	um.requestChan <- &userManagerCommand{
		rchan:       rchan,
		commandType: commandAddUser,
		payload:     []string{id, name},
	}

	umr := <-rchan

	return getUserOrError(umr)
}
