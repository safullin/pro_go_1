package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFileObserver(t *testing.T) {
	path := t.TempDir() + "/audit.log"
	observer, err := NewFileObserver(path)
	if err != nil {
		t.Fatalf("create observer: %v", err)
	}
	t.Cleanup(func() {
		if err := observer.Close(); err != nil {
			t.Errorf("close observer: %v", err)
		}
	})

	event := Event{TS: 10, Metrics: []string{"Alloc"}, IPAddress: "192.168.0.42"}
	if err := observer.Notify(context.Background(), event); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if err := observer.Close(); err != nil {
		t.Fatalf("close observer: %v", err)
	}
	if err := observer.Notify(context.Background(), event); !errors.Is(err, errFileObserverClosed) {
		t.Fatalf("notify after close: got %v want %v", err, errFileObserverClosed)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit file: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("audit record must end with newline: %q", data)
	}

	var got Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &got); err != nil {
		t.Fatalf("decode audit event: %v", err)
	}
	if got.TS != event.TS || got.IPAddress != event.IPAddress || len(got.Metrics) != 1 || got.Metrics[0] != "Alloc" {
		t.Fatalf("unexpected event: %#v", got)
	}
}

func TestHTTPObserver(t *testing.T) {
	events := make(chan Event, 1)
	var attempts atomic.Int32
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content type: %q", r.Header.Get("Content-Type"))
		}
		var event Event
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("decode event: %v", err)
			return
		}
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		events <- event
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	event := Event{TS: 20, Metrics: []string{"PollCount"}, IPAddress: "192.168.0.43"}
	if err := NewHTTPObserver(receiver.URL).Notify(context.Background(), event); err != nil {
		t.Fatalf("notify: %v", err)
	}

	got := <-events
	if got.TS != event.TS || got.IPAddress != event.IPAddress || len(got.Metrics) != 1 || got.Metrics[0] != "PollCount" {
		t.Fatalf("unexpected event: %#v", got)
	}
	if attempts.Load() != 2 {
		t.Fatalf("unexpected attempts count: got %d want 2", attempts.Load())
	}
}

func TestPublisherProcessesObserversIndependently(t *testing.T) {
	slowStarted := make(chan struct{})
	slowRelease := make(chan struct{})
	fastEvents := make(chan Event, 1)

	publisher := NewPublisher(
		observerFunc(func(_ context.Context, _ Event) error {
			close(slowStarted)
			<-slowRelease
			return nil
		}),
		observerFunc(func(_ context.Context, event Event) error {
			fastEvents <- event
			return nil
		}),
	)
	defer publisher.Close()
	defer close(slowRelease)

	event := Event{TS: 30, Metrics: []string{"Alloc"}}
	publisher.Publish(context.Background(), event)

	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("slow observer did not start")
	}
	select {
	case got := <-fastEvents:
		if got.TS != event.TS {
			t.Fatalf("unexpected event: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("fast observer was blocked")
	}
}

func TestPublisherLogsObserverErrors(t *testing.T) {
	var output bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previousOutput)

	publisher := NewPublisher(observerFunc(func(_ context.Context, _ Event) error {
		return errors.New("write failed")
	}))
	publisher.Publish(context.Background(), Event{TS: 40})
	publisher.Close()

	if !strings.Contains(output.String(), "write failed") {
		t.Fatalf("observer error was not logged: %q", output.String())
	}
}

type observerFunc func(context.Context, Event) error

func (f observerFunc) Notify(ctx context.Context, event Event) error {
	return f(ctx, event)
}
