package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/safullin/pro_go_1/internal/cryptoutil"
	"github.com/safullin/pro_go_1/internal/model"
	"github.com/safullin/pro_go_1/internal/signature"
)

type stubRuntimeReader struct {
	stats runtime.MemStats
}

func (s stubRuntimeReader) ReadMemStats(dst *runtime.MemStats) {
	*dst = s.stats
}

type stubSystemReader struct {
	metrics SystemMetrics
	err     error
}

func (s stubSystemReader) ReadSystemMetrics() (SystemMetrics, error) {
	return s.metrics, s.err
}

type retryDoer struct {
	failures int
	calls    int
}

func (d *retryDoer) Do(*http.Request) (*http.Response, error) {
	d.calls++
	if d.calls <= d.failures {
		return nil, errors.New("connection refused")
	}

	return &http.Response{
		Status:     http.StatusText(http.StatusOK),
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

type statusDoer struct {
	statusCode int
	calls      int
}

func (d *statusDoer) Do(*http.Request) (*http.Response, error) {
	d.calls++
	return &http.Response{
		Status:     http.StatusText(d.statusCode),
		StatusCode: d.statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

type blockingDoer struct {
	started     chan struct{}
	release     chan struct{}
	canceled    chan struct{}
	mu          sync.Mutex
	calls       int
	startOnce   sync.Once
	releaseOnce sync.Once
	cancelOnce  sync.Once
}

func newBlockingDoer() *blockingDoer {
	return &blockingDoer{
		started:  make(chan struct{}),
		release:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
}

func (d *blockingDoer) Do(req *http.Request) (*http.Response, error) {
	d.mu.Lock()
	d.calls++
	d.mu.Unlock()
	d.startOnce.Do(func() {
		close(d.started)
	})
	select {
	case <-d.release:
		return &http.Response{
			Status:     http.StatusText(http.StatusOK),
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	case <-req.Context().Done():
		d.cancelOnce.Do(func() {
			close(d.canceled)
		})
		return nil, req.Context().Err()
	}
}

func (d *blockingDoer) Release() {
	d.releaseOnce.Do(func() {
		close(d.release)
	})
}

func (d *blockingDoer) Calls() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

func TestRefreshMetrics(t *testing.T) {
	reader := stubRuntimeReader{
		stats: runtime.MemStats{
			Alloc:         10,
			GCCPUFraction: 0.5,
			HeapAlloc:     20,
			NumGC:         3,
			TotalAlloc:    40,
		},
	}

	metricsAgent := New("http://localhost:8080", time.Second, time.Second)
	metricsAgent.reader = reader
	metricsAgent.randomValue = func() float64 { return 99.9 }

	metricsAgent.refreshMetrics()
	metricsAgent.refreshMetrics()

	if got := metricsAgent.gauges["Alloc"]; got != 10 {
		t.Fatalf("unexpected Alloc value: got %v want %v", got, 10.0)
	}
	if got := metricsAgent.gauges["GCCPUFraction"]; got != 0.5 {
		t.Fatalf("unexpected GCCPUFraction value: got %v want %v", got, 0.5)
	}
	if got := metricsAgent.gauges[model.RandomValueMetric]; got != 99.9 {
		t.Fatalf("unexpected RandomValue value: got %v want %v", got, 99.9)
	}
	if got := metricsAgent.counters[model.PollCountMetric]; got != 2 {
		t.Fatalf("unexpected PollCount value: got %v want %v", got, 2)
	}
}

func TestRefreshSystemMetrics(t *testing.T) {
	metricsAgent := New("http://localhost:8080", time.Second, time.Second)
	metricsAgent.systemReader = stubSystemReader{
		metrics: SystemMetrics{
			TotalMemory:    1024,
			FreeMemory:     512,
			CPUUtilization: []float64{10.5, 20.5},
		},
	}

	metricsAgent.refreshSystemMetrics()

	if got := metricsAgent.gauges["TotalMemory"]; got != 1024 {
		t.Fatalf("unexpected TotalMemory value: got %v want %v", got, 1024.0)
	}
	if got := metricsAgent.gauges["FreeMemory"]; got != 512 {
		t.Fatalf("unexpected FreeMemory value: got %v want %v", got, 512.0)
	}
	if got := metricsAgent.gauges["CPUutilization1"]; got != 10.5 {
		t.Fatalf("unexpected CPUutilization1 value: got %v want %v", got, 10.5)
	}
	if got := metricsAgent.gauges["CPUutilization2"]; got != 20.5 {
		t.Fatalf("unexpected CPUutilization2 value: got %v want %v", got, 20.5)
	}
}

func TestReportMetrics(t *testing.T) {
	requests := 0
	var metrics []model.Metrics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/updates/" {
			t.Fatalf("unexpected request path: got %q want %q", r.URL.Path, "/updates/")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: got %q want %q", got, "application/json")
		}
		if got := r.Header.Get("Content-Encoding"); got != "gzip" {
			t.Fatalf("unexpected content encoding: got %q want %q", got, "gzip")
		}
		if got := r.Header.Get("X-Real-IP"); got != "192.0.2.10" {
			t.Fatalf("unexpected real IP: got %q want %q", got, "192.0.2.10")
		}

		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer zr.Close()

		if err := json.NewDecoder(zr).Decode(&metrics); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metricsAgent := New(server.URL, time.Second, time.Second)
	metricsAgent.realIP = "192.0.2.10"
	metricsAgent.gauges["Alloc"] = 100.5
	metricsAgent.counters["PollCount"] = 4

	metricsAgent.reportMetrics(context.Background())

	if requests != 1 {
		t.Fatalf("unexpected requests count: got %d want %d", requests, 1)
	}
	if len(metrics) != 2 {
		t.Fatalf("unexpected metrics count: got %d want %d", len(metrics), 2)
	}

	gotGauge := false
	gotCounter := false
	for _, metric := range metrics {
		switch metric.ID {
		case "Alloc":
			if metric.MType != model.Gauge || metric.Value == nil || *metric.Value != 100.5 {
				t.Fatalf("unexpected gauge payload: %#v", metric)
			}
			gotGauge = true
		case "PollCount":
			if metric.MType != model.Counter || metric.Delta == nil || *metric.Delta != 4 {
				t.Fatalf("unexpected counter payload: %#v", metric)
			}
			gotCounter = true
		default:
			t.Fatalf("unexpected metric payload: %#v", metric)
		}
	}

	if !gotGauge || !gotCounter {
		t.Fatalf("missing reported metrics: gauge=%v counter=%v", gotGauge, gotCounter)
	}
	if got := metricsAgent.counters["PollCount"]; got != 0 {
		t.Fatalf("counter was not reset after successful report: got %d", got)
	}
}

func TestAckCountersKeepsNewValues(t *testing.T) {
	metricsAgent := New("http://localhost:8080", time.Second, time.Second)
	metricsAgent.counters[model.PollCountMetric] = 4

	job := metricsAgent.buildReportJob()
	metricsAgent.counters[model.PollCountMetric] = 6
	metricsAgent.ackCounters(job.counters)

	if got := metricsAgent.counters[model.PollCountMetric]; got != 2 {
		t.Fatalf("unexpected counter value: got %d want %d", got, 2)
	}
}

func TestSetRateLimit(t *testing.T) {
	metricsAgent := New("http://localhost:8080", time.Second, time.Second)

	metricsAgent.SetRateLimit(3)
	if metricsAgent.rateLimit != 3 {
		t.Fatalf("unexpected rate limit: got %d want %d", metricsAgent.rateLimit, 3)
	}

	metricsAgent.SetRateLimit(0)
	if metricsAgent.rateLimit != 3 {
		t.Fatalf("unexpected rate limit after invalid value: got %d want %d", metricsAgent.rateLimit, 3)
	}
}

func TestEnqueueReportJobDropsWhenQueueFull(t *testing.T) {
	jobs := make(chan reportJob, 1)
	jobs <- reportJob{}

	done := make(chan struct{})
	go func() {
		defer close(done)
		enqueueReportJob(context.Background(), jobs, reportJob{})
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("enqueue report job blocked on full queue")
	}
	if len(jobs) != 1 {
		t.Fatalf("unexpected queue length: got %d want %d", len(jobs), 1)
	}
}

func TestReportMetricsSkipsEmptyBatch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metricsAgent := New(server.URL, time.Second, time.Second)

	metricsAgent.reportMetrics(context.Background())

	if requests != 0 {
		t.Fatalf("unexpected requests count: got %d want %d", requests, 0)
	}
}

func TestRunSendsFinalReportOnShutdown(t *testing.T) {
	client := &statusDoer{statusCode: http.StatusOK}
	metricsAgent := New("http://localhost:8080", time.Hour, time.Hour)
	metricsAgent.client = client
	metricsAgent.systemReader = stubSystemReader{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	metricsAgent.Run(ctx)

	if client.calls != 1 {
		t.Fatalf("requests = %d, want 1 final report", client.calls)
	}
	if got := metricsAgent.counters[model.PollCountMetric]; got != 0 {
		t.Fatalf("PollCount = %d, want 0 after final report", got)
	}
}

func TestRunRetriesInFlightReportOnShutdown(t *testing.T) {
	client := newBlockingDoer()
	defer client.Release()
	metricsAgent := New("http://localhost:8080", time.Hour, time.Millisecond)
	metricsAgent.client = client
	metricsAgent.systemReader = stubSystemReader{}
	metricsAgent.deliveryTimeout = time.Second

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		metricsAgent.Run(ctx)
	}()

	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("report did not start")
	}
	cancel()

	select {
	case <-client.canceled:
	case <-time.After(time.Second):
		t.Fatal("in-flight report was not canceled")
	}
	select {
	case <-runDone:
		t.Fatal("agent stopped before final report completed")
	default:
	}

	client.Release()
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("agent did not stop after reports completed")
	}
	if calls := client.Calls(); calls < 2 {
		t.Fatalf("requests = %d, want at least 2", calls)
	}
	if got := metricsAgent.counters[model.PollCountMetric]; got != 0 {
		t.Fatalf("PollCount = %d, want 0 after final report", got)
	}
}

func TestRunLimitsFinalReportDelivery(t *testing.T) {
	client := newBlockingDoer()
	defer client.Release()
	metricsAgent := New("http://localhost:8080", time.Hour, time.Hour)
	metricsAgent.client = client
	metricsAgent.systemReader = stubSystemReader{}
	metricsAgent.deliveryTimeout = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		metricsAgent.Run(ctx)
	}()

	select {
	case <-client.canceled:
	case <-time.After(time.Second):
		t.Fatal("final report context was not canceled")
	}
	select {
	case <-runDone:
	case <-time.After(time.Second):
		t.Fatal("agent did not stop after final report timeout")
	}
}

func TestSendMetricsRetriesTemporaryTransportErrors(t *testing.T) {
	client := &retryDoer{failures: 2}
	metricsAgent := New("http://localhost:8080", time.Second, time.Second)
	metricsAgent.client = client
	metricsAgent.retryDelays = []time.Duration{0, 0, 0}

	value := 100.5
	err := metricsAgent.sendMetrics(context.Background(), []model.Metrics{
		{
			ID:    "Alloc",
			MType: model.Gauge,
			Value: &value,
		},
	})
	if err != nil {
		t.Fatalf("send metrics: %v", err)
	}
	if client.calls != 3 {
		t.Fatalf("unexpected attempts count: got %d want %d", client.calls, 3)
	}
}

func TestSendMetricsDoesNotRetryStatusErrors(t *testing.T) {
	client := &statusDoer{statusCode: http.StatusBadRequest}
	metricsAgent := New("http://localhost:8080", time.Second, time.Second)
	metricsAgent.client = client
	metricsAgent.retryDelays = []time.Duration{0, 0, 0}

	value := 100.5
	err := metricsAgent.sendMetrics(context.Background(), []model.Metrics{
		{
			ID:    "Alloc",
			MType: model.Gauge,
			Value: &value,
		},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if client.calls != 1 {
		t.Fatalf("unexpected attempts count: got %d want %d", client.calls, 1)
	}
}

func TestSendMetricsSignsRequest(t *testing.T) {
	const key = "secret"

	var gotHash string
	var gotBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHash = r.Header.Get(signature.Header)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metricsAgent := New(server.URL, time.Second, time.Second, key)
	value := 100.5
	err := metricsAgent.sendMetrics(context.Background(), []model.Metrics{
		{
			ID:    "Alloc",
			MType: model.Gauge,
			Value: &value,
		},
	})
	if err != nil {
		t.Fatalf("send metrics: %v", err)
	}

	if gotHash == "" {
		t.Fatal("expected hash header, got empty")
	}
	if !signature.Valid(gotBody, key, gotHash) {
		t.Fatalf("invalid hash header: %q", gotHash)
	}
}

func TestSendMetricsEncryptsRequest(t *testing.T) {
	const key = "secret"
	privateKey := generateAgentRSAKey(t)
	var received []model.Metrics

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(cryptoutil.Header); got != cryptoutil.Algorithm {
			t.Fatalf("encryption header = %q, want %q", got, cryptoutil.Algorithm)
		}
		encrypted, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read encrypted body: %v", err)
		}
		compressed, err := cryptoutil.Decrypt(encrypted, privateKey)
		if err != nil {
			t.Fatalf("decrypt body: %v", err)
		}
		if !signature.Valid(compressed, key, r.Header.Get(signature.Header)) {
			t.Fatal("request signature is invalid")
		}

		reader, err := gzip.NewReader(bytes.NewReader(compressed))
		if err != nil {
			t.Fatalf("open gzip body: %v", err)
		}
		defer reader.Close()
		if err := json.NewDecoder(reader).Decode(&received); err != nil {
			t.Fatalf("decode metrics: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metricsAgent := New(server.URL, time.Second, time.Second, key)
	metricsAgent.SetPublicKey(&privateKey.PublicKey)
	value := 100.5
	err := metricsAgent.sendMetrics(context.Background(), []model.Metrics{
		{
			ID:    "Alloc",
			MType: model.Gauge,
			Value: &value,
		},
	})
	if err != nil {
		t.Fatalf("send metrics: %v", err)
	}
	if len(received) != 1 || received[0].ID != "Alloc" {
		t.Fatalf("received metrics = %#v", received)
	}
}

func generateAgentRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}
