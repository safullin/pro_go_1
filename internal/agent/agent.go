package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"github.com/safullin/pro_go_1/internal/model"
)

// HTTPDoer описывает клиент, умеющий отправлять HTTP-запросы.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// RuntimeReader позволяет подменять runtime.ReadMemStats в тестах.
type RuntimeReader interface {
	ReadMemStats(stats *runtime.MemStats)
}

type runtimeReader struct{}

func (runtimeReader) ReadMemStats(stats *runtime.MemStats) {
	runtime.ReadMemStats(stats)
}

// Agent собирает runtime-метрики и отправляет их на сервер.
type Agent struct {
	address        string
	pollInterval   time.Duration
	reportInterval time.Duration
	client         HTTPDoer
	reader         RuntimeReader
	randomValue    func() float64
	gauges         map[string]float64
	counters       map[string]int64
}

// New создаёт нового агента.
func New(address string, pollInterval, reportInterval time.Duration) *Agent {
	return &Agent{
		address:        address,
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		reader:      runtimeReader{},
		randomValue: rand.Float64,
		gauges:      make(map[string]float64),
		counters:    make(map[string]int64),
	}
}

// Run запускает циклы обновления и отправки метрик.
func (a *Agent) Run(ctx context.Context) {
	a.refreshMetrics()

	pollTicker := time.NewTicker(a.pollInterval)
	reportTicker := time.NewTicker(a.reportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			a.refreshMetrics()
		case <-reportTicker.C:
			a.reportMetrics(ctx)
		}
	}
}

func (a *Agent) refreshMetrics() {
	var stats runtime.MemStats
	a.reader.ReadMemStats(&stats)

	a.gauges["Alloc"] = float64(stats.Alloc)
	a.gauges["BuckHashSys"] = float64(stats.BuckHashSys)
	a.gauges["Frees"] = float64(stats.Frees)
	a.gauges["GCCPUFraction"] = stats.GCCPUFraction
	a.gauges["GCSys"] = float64(stats.GCSys)
	a.gauges["HeapAlloc"] = float64(stats.HeapAlloc)
	a.gauges["HeapIdle"] = float64(stats.HeapIdle)
	a.gauges["HeapInuse"] = float64(stats.HeapInuse)
	a.gauges["HeapObjects"] = float64(stats.HeapObjects)
	a.gauges["HeapReleased"] = float64(stats.HeapReleased)
	a.gauges["HeapSys"] = float64(stats.HeapSys)
	a.gauges["LastGC"] = float64(stats.LastGC)
	a.gauges["Lookups"] = float64(stats.Lookups)
	a.gauges["MCacheInuse"] = float64(stats.MCacheInuse)
	a.gauges["MCacheSys"] = float64(stats.MCacheSys)
	a.gauges["MSpanInuse"] = float64(stats.MSpanInuse)
	a.gauges["MSpanSys"] = float64(stats.MSpanSys)
	a.gauges["Mallocs"] = float64(stats.Mallocs)
	a.gauges["NextGC"] = float64(stats.NextGC)
	a.gauges["NumForcedGC"] = float64(stats.NumForcedGC)
	a.gauges["NumGC"] = float64(stats.NumGC)
	a.gauges["OtherSys"] = float64(stats.OtherSys)
	a.gauges["PauseTotalNs"] = float64(stats.PauseTotalNs)
	a.gauges["StackInuse"] = float64(stats.StackInuse)
	a.gauges["StackSys"] = float64(stats.StackSys)
	a.gauges["Sys"] = float64(stats.Sys)
	a.gauges["TotalAlloc"] = float64(stats.TotalAlloc)
	a.gauges[model.RandomValueMetric] = a.randomValue()
	a.counters[model.PollCountMetric]++
}

func (a *Agent) reportMetrics(ctx context.Context) {
	for name, value := range a.gauges {
		value := value
		_ = a.sendMetric(ctx, model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &value,
		})
	}
	for name, value := range a.counters {
		value := value
		_ = a.sendMetric(ctx, model.Metrics{
			ID:    name,
			MType: model.Counter,
			Delta: &value,
		})
	}
}

func (a *Agent) sendMetric(ctx context.Context, metric model.Metrics) error {
	body, err := json.Marshal(metric)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.address+"/update/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}
