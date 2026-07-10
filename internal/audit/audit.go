package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Event описывает событие аудита полученных метрик.
type Event struct {
	TS        int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// Observer принимает события аудита.
type Observer interface {
	// Notify сохраняет событие аудита.
	Notify(ctx context.Context, event Event) error
}

// Publisher передаёт событие всем зарегистрированным наблюдателям.
type Publisher struct {
	observers []Observer
}

// NewPublisher создаёт издателя событий аудита.
func NewPublisher(observers ...Observer) *Publisher {
	return &Publisher{observers: observers}
}

// Publish передаёт событие всем наблюдателям.
func (p *Publisher) Publish(ctx context.Context, event Event) {
	for _, observer := range p.observers {
		_ = observer.Notify(ctx, event)
	}
}

// FileObserver сохраняет события аудита в файл.
type FileObserver struct {
	mu   sync.Mutex
	path string
}

// NewFileObserver создаёт наблюдателя, записывающего события в path.
func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

// Notify добавляет событие в файл отдельной строкой.
func (o *FileObserver) Notify(_ context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	file, err := os.OpenFile(o.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(append(data, '\n'))
	return err
}

// HTTPObserver отправляет события аудита по HTTP.
type HTTPObserver struct {
	client *http.Client
	url    string
}

// NewHTTPObserver создаёт наблюдателя для отправки событий на url.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Notify отправляет событие POST-запросом.
func (o *HTTPObserver) Notify(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}
