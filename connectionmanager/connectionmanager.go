//Package connectionmanager provides communication between connections who are
//connected in different goroutines.
package connectionmanager

import (
	"container/list"
	"errors"
	"fmt"
	//"log"
)

// TODO: helper functions for SendMessage?
// TODO: allow user to turn polling off explicitly
// TODO: allow sending arbitrary messages with no sender?
// TODO: unicast message
// TODO: multicast message (register multicast groups?)
// TODO: rework main thread to use functions
// TODO: have general storage for the Connection so we don't have setConnectionName, setAge, etc.

// Message type identifiers
type MessageType int32

const (
	StopRequest       MessageType = 0
	StopResponse      MessageType = 1
	ConnectRequest    MessageType = 2
	ConnectResponse   MessageType = 3
	BroadcastRequest  MessageType = 4
	BroadcastResponse MessageType = 5
	PollRequest       MessageType = 6
	PollResponse      MessageType = 7
	Broadcast         MessageType = 8
)

// This dictates how many reentrant calls to SendRequest() can be made
// from a single thread without deadlocking (as if the thread made a
// second call to SendRequest() while servicing a side effect of a first
// call). I would not expect more than 3 in normal usage.
const messageChannelSize = 50

// Information about a particular connection
type Connection struct {
	// unique ID (UUID-ish) associated with this connection
	id string

	// channel for receiving messages for this connection
	pollChannel chan *[]*Message

	// true if the connection is polling 
	polling bool

	// undelivered messages
	messages *list.List
}

// Message payload for Message struct
type MessagePayload map[string]interface{}

// Messages to and from the ConnectionManager
type Message struct {
	// Type of message
	Type MessageType

	// ID associated with this message
	Id string

	// ID of recipient
	DestId string

	// Additional payload to be passed to recipient (or broadcast)
	Payload *MessagePayload

	// Channel for response
	RChan chan *Message

	// Channel for polling
	PollChan chan *[]*Message

	// Generic field for data passing
	General interface{}

	// Status for SendMessage return
	Err error
}

// Manages connections
type ConnectionManager struct {
	// list of connections
	connection map[string]*Connection

	// the ConnectionManager's incoming message channel
	messageChannel chan *Message

	// true if the handler routine is running
	active bool
}

// Check if a connection is polling, and send responses
func (c *Connection) pollCheck() {
	l := c.messages.Len()
	if c.polling && l > 0 {
		// Array for passing messages to poller
		// (poller will own)
		messageArray := make([]*Message, l)

		// make new references to the data
		count := 0
		for e := c.messages.Front(); e != nil; e = e.Next() {
			messageArray[count] = e.Value.(*Message)
			count++
		}

		// ditch sent messages
		c.messages.Init()

		//log.Printf("ConnectionManager: pollCheck: sending to %s: %v\n", c.id, c.messages)
		c.pollChannel <- &messageArray
		//log.Printf("ConnectionManager: pollCheck: sending to %s: complete\n", c.id)

		// unmark connections as polling
		c.polling = false
	}
}

// Allocate and initialize a new connection
func newConnection(id string) *Connection {
	connection := &Connection{
		pollChannel: nil, // will be set when the connection starts polling
		polling:     false,
		messages:    list.New(),
		id:          id,
	}

	return connection
}

// remove a connection from tracking
func (cm *ConnectionManager) removeConnection(connection *Connection) {
	// TODO
}

// Start or stop a connection manager service
func (cm *ConnectionManager) SetActive(active bool) {
	if active {
		if !cm.active {
			go runConnectionManager(cm)
			cm.active = true
		}
	} else {
		if cm.active {
			cm.SendMessage(&Message{
				Type: StopRequest,
			})
		}
	}
}

// Create a new ConnectionManager
func New() *ConnectionManager {
	cm := &ConnectionManager{
		connection:     make(map[string]*Connection),
		messageChannel: make(chan *Message, messageChannelSize),
	}

	return cm
}

// Returns exportable connection data
/*
func (cm *ConnectionManager) buildConnectionListResponse() []map[string]interface{} {
	// make an array of maps
	r := make([]map[string]interface{}, len(cm.connection))

	i := 0
	for _, c := range cm.connection {
		r[i] = map[string]interface{}{
			"id":       c.pubId,
			"userdata": c.attr,
		}
		i++
	}

	return r
}
*/

// Sends a Message and receives a Message
//
// to be called from other threads
func (cm *ConnectionManager) SendMessage(r *Message) *Message {
	r.RChan = make(chan *Message)

	//log.Printf("SendMessage: sending %s\n", *r)
	cm.messageChannel <- r
	//log.Printf("SendMessage: receiving\n")
	resp := <-r.RChan
	//log.Printf("SendMessage: received %s\n", *resp)

	close(r.RChan) // necessary?
	r.RChan = nil

	return resp
}

// Broadcasts a response to all connections
func (cm *ConnectionManager) broadcast(r *Message) {
	for _, c := range cm.connection {
		// buffer to all connections
		c.messages.PushBack(r)

		// send if polling
		c.pollCheck()
	}
}

// Handle a ConnectRequest Message
func (cm *ConnectionManager) handleConnectRequest(m *Message) {
	var c *Connection
	var present bool

	// make a new connection if we don't have it
	if c, present = cm.connection[m.Id]; !present {
		c = newConnection(m.Id)

		//log.Printf("ConnectionManager: %s: new connection\n", m.Id)
	}

	// add to the list
	cm.connection[m.Id] = c

	// send response
	//log.Println("ConnectionManager: sending login response")
	m.RChan <- &Message{
		Type: ConnectResponse,
		Id:   m.Id,
		Err:  nil,
	}
	//log.Println("ConnectionManager: sent login response")
}

// Handle a StopRequest Message
func (cm *ConnectionManager) handleStopRequest(m *Message) {
	//log.Println("ConnectionManager: sending stop response")

	m.RChan <- &Message{
		Type: StopResponse,
		Err:  nil,
	}

	//log.Println("ConnectionManager: sent stop response")
}

// Handle a PollRequest Message
//
// This will cause queued messages to be delivered, if any exist.
func (cm *ConnectionManager) handlePollRequest(m *Message) {
	c, ok := cm.connection[m.Id]

	if !ok {
		//log.Printf("ConnectionManager: unknown user ID for PollMessage: %s\n", m.Id)

		m.RChan <- &Message{
			Type: PollResponse,
			Err:  errors.New(fmt.Sprintf("PollRequest: unknown user id: %s", m.Id)),
		}

		return
	}

	// If the connection is already polling, it means they've opened a
	// new polling connection. The old one needs to be shut down
	// before we can continue
	if c.polling {
		close(c.pollChannel)
	}

	// mark connection as polling
	c.polling = true
	c.pollChannel = make(chan *[]*Message)

	//log.Println("ConnectionManager: sending pollmessage response")

	m.RChan <- &Message{
		Type:     PollResponse,
		PollChan: c.pollChannel,
		Err:      nil,
	}

	//log.Println("ConnectionManager: sent pollmessage response")

	// push if we already have something
	c.pollCheck()
}

// Handle a BroadcastRequest Message
//
// Message.Payload should be set to something useful
//
// Warning: changes m.Type to Broadcast
func (cm *ConnectionManager) handleBroadcastRequest(m *Message) {
	// change type from BroadcastRequest to Broadcast
	m.Type = Broadcast

	// buffer messages and push to waiting connections
	cm.broadcast(m)

	//log.Println("ConnectionManager: sending broadcast response")

	m.RChan <- &Message{
		Type: BroadcastResponse,
		Err:  nil,
	}

	//log.Println("ConnectionManager: sent broadcast response")
}

// Manages connections (runs as a goroutine)
func runConnectionManager(cm *ConnectionManager) {
	var message *Message

	for {
		//log.Printf("ConnectionManager: waiting for message %v %v", cm, cm.messageChannel)

		message = <- cm.messageChannel

		//log.Printf("ConnectionManager: got message: %s\n", message)

		switch message.Type {

		case ConnectRequest:
			cm.handleConnectRequest(message)

		case StopRequest:
			cm.handleStopRequest(message)
			return // exit from the goroutine

		case PollRequest:
			cm.handlePollRequest(message)

		case BroadcastRequest:
			cm.handleBroadcastRequest(message)

		default:
			panic(fmt.Sprintf("Unknown message: \"%d\"", message.Type))
		}

		//log.Println("ConnectionManager: finished servicing message")
	}
}
