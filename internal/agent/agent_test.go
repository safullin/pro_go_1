package agent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

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
