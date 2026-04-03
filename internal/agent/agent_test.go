package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
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
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if got := r.Header.Get("Content-Type"); got != "text/plain" {
			t.Fatalf("unexpected content type: got %q want %q", got, "text/plain")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	metricsAgent := New(server.URL, time.Second, time.Second)
	metricsAgent.gauges["Alloc"] = 100.5
	metricsAgent.counters["PollCount"] = 4

	metricsAgent.reportMetrics(context.Background())

	sort.Strings(paths)
	want := []string{
		"/update/counter/PollCount/4",
		"/update/gauge/Alloc/100.5",
	}

	if len(paths) != len(want) {
		t.Fatalf("unexpected requests count: got %d want %d", len(paths), len(want))
	}

	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("unexpected request path: got %q want %q", paths[i], want[i])
		}
	}
}
