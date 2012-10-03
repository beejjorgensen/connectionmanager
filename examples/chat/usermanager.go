// User Manager code for chat.go
package main

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"sync"
)

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

	// Mutexes
	userMutex            *sync.RWMutex
	nextGuestNumberMutex *sync.Mutex
}

// Construct a new UserManager
func newUserManager() *UserManager {
	userManager := &UserManager{
		user:                 make(map[string]*User),
		idMap:                make(map[string]string),
		nextGuestNumber:      1,
		userMutex:            new(sync.RWMutex),
		nextGuestNumberMutex: new(sync.Mutex),
	}

	return userManager
}

// Autogenerate a unique printable username
func (um *UserManager) nextGuestName() string {
	um.nextGuestNumberMutex.Lock()
	defer um.nextGuestNumberMutex.Unlock()

	name := fmt.Sprintf("Guest%d", um.nextGuestNumber)
	um.nextGuestNumber++

	return name
}

// Add a new user
//
// Returns pointer to user
func (um *UserManager) addUser(id string, name string) *User {
	um.userMutex.Lock()
	defer um.userMutex.Unlock()

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
	um.userMutex.RLock()
	defer um.userMutex.RUnlock()

	u, ok := um.user[id]

	return u, ok
}

// Return a user by PubID
func (um *UserManager) getUserByPubId(pid string) (*User, bool) {
	um.userMutex.RLock()
	defer um.userMutex.RUnlock()

	id, ok := um.idMap[pid]
	if !ok {
		return nil, ok
	}

	u, ok := um.user[id]
	return u, ok
}
