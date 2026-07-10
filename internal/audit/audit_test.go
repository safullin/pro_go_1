package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestFileObserver(t *testing.T) {
	path := t.TempDir() + "/audit.log"
	observer := NewFileObserver(path)

	event := Event{TS: 10, Metrics: []string{"Alloc"}, IPAddress: "192.168.0.42"}
	if err := observer.Notify(context.Background(), event); err != nil {
		t.Fatalf("notify: %v", err)
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
}
