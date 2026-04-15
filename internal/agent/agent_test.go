package agent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/safullin/pro_go_1/internal/model"
)

type stubRuntimeReader struct {
	stats runtime.MemStats
}

func (s stubRuntimeReader) ReadMemStats(dst *runtime.MemStats) {
	*dst = s.stats
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

func TestReportMetrics(t *testing.T) {
	var metrics []model.Metrics
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/update/" {
			t.Fatalf("unexpected request path: got %q want %q", r.URL.Path, "/update/")
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

		var metric model.Metrics
		if err := json.NewDecoder(zr).Decode(&metric); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		metrics = append(metrics, metric)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metricsAgent := New(server.URL, time.Second, time.Second)
	metricsAgent.gauges["Alloc"] = 100.5
	metricsAgent.counters["PollCount"] = 4

	metricsAgent.reportMetrics(context.Background())

	if len(metrics) != 2 {
		t.Fatalf("unexpected requests count: got %d want %d", len(metrics), 2)
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
}
