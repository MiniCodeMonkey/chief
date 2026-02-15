package ws

import (
	"encoding/json"
	"log"
	"sync"
)

// Handler processes a received WebSocket message.
// The raw JSON is provided so handlers can unmarshal into the specific message type.
type Handler func(msg Message, raw json.RawMessage)

// Dispatcher routes incoming WebSocket messages to registered handlers by message type.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// NewDispatcher creates a new message dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string]Handler),
	}
}

// Register registers a handler for the given message type.
// If a handler is already registered for the type, it is replaced.
func (d *Dispatcher) Register(msgType string, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[msgType] = handler
}

// Unregister removes the handler for the given message type.
func (d *Dispatcher) Unregister(msgType string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.handlers, msgType)
}

// Dispatch routes a message to its registered handler.
// Unknown message types are logged and ignored (forward compatibility).
// Returns true if a handler was found and called, false otherwise.
func (d *Dispatcher) Dispatch(msg Message) bool {
	d.mu.RLock()
	handler, ok := d.handlers[msg.Type]
	d.mu.RUnlock()

	if !ok {
		log.Printf("ws: no handler for message type %q, ignoring", msg.Type)
		return false
	}

	handler(msg, msg.Raw)
	return true
}
