package ws

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestDispatcherRegisterAndDispatch(t *testing.T) {
	d := NewDispatcher()

	var called bool
	var receivedType string
	d.Register(TypeListProjects, func(msg Message, raw json.RawMessage) {
		called = true
		receivedType = msg.Type
	})

	msg := Message{Type: TypeListProjects, ID: "test-id", Timestamp: "2026-02-15T10:00:00Z"}
	handled := d.Dispatch(msg)

	if !handled {
		t.Error("expected Dispatch to return true")
	}
	if !called {
		t.Error("handler was not called")
	}
	if receivedType != TypeListProjects {
		t.Errorf("received type = %q, want %q", receivedType, TypeListProjects)
	}
}

func TestDispatcherUnknownType(t *testing.T) {
	d := NewDispatcher()

	msg := Message{Type: "unknown_message_type", ID: "test-id", Timestamp: "2026-02-15T10:00:00Z"}
	handled := d.Dispatch(msg)

	if handled {
		t.Error("expected Dispatch to return false for unknown type")
	}
}

func TestDispatcherUnregister(t *testing.T) {
	d := NewDispatcher()

	var callCount int
	d.Register(TypeGetProject, func(msg Message, raw json.RawMessage) {
		callCount++
	})

	msg := Message{Type: TypeGetProject, ID: "test-id"}
	d.Dispatch(msg)
	if callCount != 1 {
		t.Fatalf("call count = %d, want 1", callCount)
	}

	d.Unregister(TypeGetProject)
	handled := d.Dispatch(msg)
	if handled {
		t.Error("expected false after unregister")
	}
	if callCount != 1 {
		t.Errorf("call count = %d, want 1 (should not have been called again)", callCount)
	}
}

func TestDispatcherReplaceHandler(t *testing.T) {
	d := NewDispatcher()

	var firstCalled, secondCalled bool
	d.Register(TypeStartRun, func(msg Message, raw json.RawMessage) {
		firstCalled = true
	})
	d.Register(TypeStartRun, func(msg Message, raw json.RawMessage) {
		secondCalled = true
	})

	d.Dispatch(Message{Type: TypeStartRun})

	if firstCalled {
		t.Error("first handler should not have been called after replacement")
	}
	if !secondCalled {
		t.Error("second handler should have been called")
	}
}

func TestDispatcherPassesRawJSON(t *testing.T) {
	d := NewDispatcher()

	var receivedRaw json.RawMessage
	d.Register(TypeStartRun, func(msg Message, raw json.RawMessage) {
		receivedRaw = raw
	})

	original := StartRunMessage{
		Type:      TypeStartRun,
		ID:        newUUID(),
		Timestamp: "2026-02-15T10:00:00Z",
		Project:   "my-project",
		PRDID:     "auth",
	}

	data, _ := json.Marshal(original)
	msg := Message{Type: TypeStartRun, ID: original.ID, Raw: data}
	d.Dispatch(msg)

	// The handler should be able to unmarshal the raw JSON into the specific type.
	var got StartRunMessage
	if err := json.Unmarshal(receivedRaw, &got); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if got.Project != "my-project" {
		t.Errorf("project = %q, want %q", got.Project, "my-project")
	}
	if got.PRDID != "auth" {
		t.Errorf("prd_id = %q, want %q", got.PRDID, "auth")
	}
}

func TestDispatcherConcurrentAccess(t *testing.T) {
	d := NewDispatcher()

	var mu sync.Mutex
	callCounts := make(map[string]int)

	for _, typ := range []string{TypeListProjects, TypeGetProject, TypeStartRun} {
		msgType := typ
		d.Register(msgType, func(msg Message, raw json.RawMessage) {
			mu.Lock()
			callCounts[msg.Type]++
			mu.Unlock()
		})
	}

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			types := []string{TypeListProjects, TypeGetProject, TypeStartRun}
			msg := Message{Type: types[i%3], ID: newUUID()}
			d.Dispatch(msg)
		}(i)
	}
	wg.Wait()

	total := 0
	for _, count := range callCounts {
		total += count
	}
	if total != 30 {
		t.Errorf("total dispatches = %d, want 30", total)
	}
}

func TestDispatcherMultipleHandlers(t *testing.T) {
	d := NewDispatcher()

	var pingCalled, pongCalled bool
	d.Register(TypePing, func(msg Message, raw json.RawMessage) {
		pingCalled = true
	})
	d.Register(TypePong, func(msg Message, raw json.RawMessage) {
		pongCalled = true
	})

	d.Dispatch(Message{Type: TypePing})
	d.Dispatch(Message{Type: TypePong})

	if !pingCalled {
		t.Error("ping handler not called")
	}
	if !pongCalled {
		t.Error("pong handler not called")
	}
}
