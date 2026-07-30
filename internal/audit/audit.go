package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/safullin/pro_go_1/internal/retry"
)

const observerQueueSize = 64

var errFileObserverClosed = errors.New("audit file observer is closed")

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
	mu      sync.RWMutex
	workers []observerWorker
	wg      sync.WaitGroup
	closed  bool
}

type notification struct {
	ctx   context.Context
	event Event
}

type observerWorker struct {
	observer Observer
	events   chan notification
}

// NewPublisher создаёт издателя событий аудита.
func NewPublisher(observers ...Observer) *Publisher {
	publisher := &Publisher{workers: make([]observerWorker, len(observers))}
	for i, observer := range observers {
		publisher.workers[i] = observerWorker{
			observer: observer,
			events:   make(chan notification, observerQueueSize),
		}
		publisher.wg.Add(1)
		go publisher.run(&publisher.workers[i])
	}
	return publisher
}

// Publish передаёт событие всем наблюдателям.
func (p *Publisher) Publish(ctx context.Context, event Event) {
	event.Metrics = append([]string(nil), event.Metrics...)
	message := notification{ctx: context.WithoutCancel(ctx), event: event}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return
	}

	for i := range p.workers {
		select {
		case p.workers[i].events <- message:
		default:
			log.Printf("audit observer %T queue is full, event dropped", p.workers[i].observer)
		}
	}
}

// Close завершает обработку поставленных в очередь событий.
func (p *Publisher) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	for i := range p.workers {
		close(p.workers[i].events)
	}
	p.mu.Unlock()

	p.wg.Wait()
}

func (p *Publisher) run(worker *observerWorker) {
	defer p.wg.Done()
	for message := range worker.events {
		if err := worker.observer.Notify(message.ctx, message.event); err != nil {
			log.Printf("audit observer %T failed: %v", worker.observer, err)
		}
	}
}

// FileObserver сохраняет события аудита в файл.
type FileObserver struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileObserver создаёт наблюдателя, записывающего события в path.
func NewFileObserver(path string) (*FileObserver, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &FileObserver{file: file}, nil
}

// Notify добавляет событие в файл отдельной строкой.
func (o *FileObserver) Notify(_ context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.file == nil {
		return errFileObserverClosed
	}

	_, err = o.file.Write(append(data, '\n'))
	return err
}

// Close закрывает файл аудита.
func (o *FileObserver) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.file == nil {
		return nil
	}
	err := o.file.Close()
	o.file = nil
	return err
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type retryClient struct {
	client httpDoer
	delays []time.Duration
}

func (c *retryClient) Do(req *http.Request) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		attemptReq := req
		if attempt > 0 {
			var err error
			attemptReq, err = cloneRequest(req)
			if err != nil {
				return nil, err
			}
		}

		resp, err := c.client.Do(attemptReq)
		if err == nil && !retryableStatus(resp.StatusCode) {
			return resp, nil
		}
		if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			if resp != nil {
				return nil, errors.Join(err, discardAndClose(resp))
			}
			return nil, err
		}
		if attempt >= len(c.delays) {
			if err != nil && resp != nil {
				return nil, errors.Join(err, discardAndClose(resp))
			}
			return resp, err
		}
		if resp != nil {
			if err := discardAndClose(resp); err != nil {
				return nil, err
			}
		}
		if err := retry.Sleep(req.Context(), c.delays[attempt]); err != nil {
			return nil, err
		}
	}
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body == nil {
		return clone, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("request body cannot be replayed")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	clone.Body = body
	return clone, nil
}

func retryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= http.StatusInternalServerError
}

func discardAndClose(resp *http.Response) error {
	_, copyErr := io.Copy(io.Discard, resp.Body)
	return errors.Join(copyErr, resp.Body.Close())
}

// HTTPObserver отправляет события аудита по HTTP.
type HTTPObserver struct {
	client httpDoer
	url    string
}

// NewHTTPObserver создаёт наблюдателя для отправки событий на url.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url: url,
		client: &retryClient{
			client: &http.Client{Timeout: 5 * time.Second},
			delays: []time.Duration{100 * time.Millisecond, 300 * time.Millisecond, 500 * time.Millisecond},
		},
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
	if err := discardAndClose(resp); err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("audit receiver returned status %d", resp.StatusCode)
	}
	return nil
}
