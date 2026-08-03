package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/safullin/pro_go_1/internal/cryptoutil"
	"github.com/safullin/pro_go_1/internal/model"
	"github.com/safullin/pro_go_1/internal/retry"
	"github.com/safullin/pro_go_1/internal/signature"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
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

// SystemMetrics хранит метрики операционной системы.
type SystemMetrics struct {
	TotalMemory    uint64
	FreeMemory     uint64
	CPUUtilization []float64
}

// SystemReader позволяет подменять gopsutil в тестах.
type SystemReader interface {
	ReadSystemMetrics() (SystemMetrics, error)
}

type gopsutilSystemReader struct{}

func (gopsutilSystemReader) ReadSystemMetrics() (SystemMetrics, error) {
	virtualMemory, err := mem.VirtualMemory()
	if err != nil {
		return SystemMetrics{}, err
	}

	cpuUtilization, err := cpu.Percent(0, true)
	if err != nil {
		return SystemMetrics{}, err
	}

	return SystemMetrics{
		TotalMemory:    virtualMemory.Total,
		FreeMemory:     virtualMemory.Free,
		CPUUtilization: cpuUtilization,
	}, nil
}

type reportJob struct {
	metrics  []model.Metrics
	counters map[string]int64
}

const (
	reportQueueMultiplier  = 2
	defaultDeliveryTimeout = 30 * time.Second
)

// Agent собирает runtime-метрики и отправляет их на сервер.
type Agent struct {
	address         string
	pollInterval    time.Duration
	reportInterval  time.Duration
	key             string
	publicKey       *rsa.PublicKey
	rateLimit       int
	retryDelays     []time.Duration
	deliveryTimeout time.Duration
	client          HTTPDoer
	reader          RuntimeReader
	systemReader    SystemReader
	randomValue     func() float64
	mu              sync.RWMutex
	gauges          map[string]float64
	counters        map[string]int64
}

// New создаёт нового агента.
func New(address string, pollInterval, reportInterval time.Duration, keys ...string) *Agent {
	var key string
	if len(keys) > 0 {
		key = keys[0]
	}

	return &Agent{
		address:         address,
		pollInterval:    pollInterval,
		reportInterval:  reportInterval,
		key:             key,
		rateLimit:       1,
		retryDelays:     []time.Duration{time.Second, 3 * time.Second, 5 * time.Second},
		deliveryTimeout: defaultDeliveryTimeout,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		reader:       runtimeReader{},
		systemReader: gopsutilSystemReader{},
		randomValue:  rand.Float64,
		gauges:       make(map[string]float64),
		counters:     make(map[string]int64),
	}
}

// SetPublicKey задаёт публичный ключ для шифрования запросов.
func (a *Agent) SetPublicKey(key *rsa.PublicKey) {
	a.publicKey = key
}

// SetRateLimit задаёт максимальное число одновременных исходящих запросов.
func (a *Agent) SetRateLimit(limit int) {
	if limit <= 0 {
		return
	}
	a.rateLimit = limit
}

// Run запускает циклы обновления и отправки метрик.
func (a *Agent) Run(ctx context.Context) {
	a.refreshMetrics()
	a.refreshSystemMetrics()

	jobs := make(chan reportJob, a.rateLimit*reportQueueMultiplier)
	var workers sync.WaitGroup
	for i := 0; i < a.rateLimit; i++ {
		workers.Add(1)
		go a.reportWorker(ctx, &workers, jobs)
	}

	var collectors sync.WaitGroup
	collectors.Add(2)
	go func() {
		defer collectors.Done()
		a.collectRuntimeMetrics(ctx)
	}()
	go func() {
		defer collectors.Done()
		a.collectSystemMetrics(ctx)
	}()

	a.reportLoop(ctx, jobs)
	collectors.Wait()
	close(jobs)
	workers.Wait()

	deliveryCtx, cancelDelivery := context.WithTimeout(context.WithoutCancel(ctx), a.deliveryTimeout)
	defer cancelDelivery()
	finalJob := a.buildReportJob()
	if len(finalJob.metrics) > 0 {
		a.sendReportJob(deliveryCtx, finalJob)
	}
}

func (a *Agent) collectRuntimeMetrics(ctx context.Context) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.refreshMetrics()
		}
	}
}

func (a *Agent) collectSystemMetrics(ctx context.Context) {
	ticker := time.NewTicker(a.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.refreshSystemMetrics()
		}
	}
}

func (a *Agent) reportLoop(ctx context.Context, jobs chan<- reportJob) {
	reportTicker := time.NewTicker(a.reportInterval)
	defer reportTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reportTicker.C:
			job := a.buildReportJob()
			if len(job.metrics) == 0 {
				continue
			}
			if !enqueueReportJob(ctx, jobs, job) && ctx.Err() != nil {
				return
			}
		}
	}
}

func enqueueReportJob(ctx context.Context, jobs chan<- reportJob, job reportJob) bool {
	select {
	case <-ctx.Done():
		return false
	case jobs <- job:
		return true
	default:
		log.Print("drop metrics batch: report queue is full")
		return false
	}
}

func (a *Agent) reportWorker(ctx context.Context, wg *sync.WaitGroup, jobs <-chan reportJob) {
	defer wg.Done()

	for job := range jobs {
		a.sendReportJob(ctx, job)
	}
}

func (a *Agent) refreshMetrics() {
	var stats runtime.MemStats
	a.reader.ReadMemStats(&stats)

	a.mu.Lock()
	defer a.mu.Unlock()

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

func (a *Agent) refreshSystemMetrics() {
	metrics, err := a.systemReader.ReadSystemMetrics()
	if err != nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.gauges["TotalMemory"] = float64(metrics.TotalMemory)
	a.gauges["FreeMemory"] = float64(metrics.FreeMemory)
	for name := range a.gauges {
		if strings.HasPrefix(name, "CPUutilization") {
			delete(a.gauges, name)
		}
	}
	for i, value := range metrics.CPUUtilization {
		a.gauges[fmt.Sprintf("CPUutilization%d", i+1)] = value
	}
}

func (a *Agent) reportMetrics(ctx context.Context) {
	job := a.buildReportJob()
	if len(job.metrics) == 0 {
		return
	}
	a.sendReportJob(ctx, job)
}

func (a *Agent) buildReportJob() reportJob {
	a.mu.RLock()
	defer a.mu.RUnlock()

	metrics := make([]model.Metrics, 0, len(a.gauges)+len(a.counters))
	for name, value := range a.gauges {
		value := value
		metrics = append(metrics, model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &value,
		})
	}
	for name, value := range a.counters {
		if value == 0 {
			continue
		}
		value := value
		metrics = append(metrics, model.Metrics{
			ID:    name,
			MType: model.Counter,
			Delta: &value,
		})
	}

	return reportJob{
		metrics:  metrics,
		counters: copyCounters(a.counters),
	}
}

func (a *Agent) sendReportJob(ctx context.Context, job reportJob) {
	if err := a.sendMetrics(ctx, job.metrics); err != nil {
		return
	}
	a.ackCounters(job.counters)
}

func (a *Agent) ackCounters(sent map[string]int64) {
	if len(sent) == 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for name, value := range sent {
		current := a.counters[name]
		if current <= value {
			a.counters[name] = 0
			continue
		}
		a.counters[name] = current - value
	}
}

func copyCounters(counters map[string]int64) map[string]int64 {
	copied := make(map[string]int64, len(counters))
	for name, value := range counters {
		if value == 0 {
			continue
		}
		copied[name] = value
	}
	return copied
}

func (a *Agent) sendMetrics(ctx context.Context, metrics []model.Metrics) error {
	body, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	compressedBody, err := gzipData(body)
	if err != nil {
		return err
	}
	requestBody := compressedBody
	if a.publicKey != nil {
		requestBody, err = cryptoutil.Encrypt(compressedBody, a.publicKey)
		if err != nil {
			return err
		}
	}

	return a.doWithRetry(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.address+"/updates/", bytes.NewReader(requestBody))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		if a.publicKey != nil {
			req.Header.Set(cryptoutil.Header, cryptoutil.Algorithm)
		}
		if a.key != "" {
			req.Header.Set(signature.Header, signature.Sum(compressedBody, a.key))
		}

		resp, err := a.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode >= http.StatusBadRequest {
			return serverStatusError{status: resp.Status}
		}
		return nil
	})
}

func (a *Agent) doWithRetry(ctx context.Context, operation func() error) error {
	err := operation()
	for _, delay := range a.retryDelays {
		if err == nil || !isRetriableAgentError(err) {
			return err
		}
		if err := retry.Sleep(ctx, delay); err != nil {
			return err
		}
		err = operation()
	}
	return err
}

func isRetriableAgentError(err error) bool {
	if err == nil {
		return false
	}
	var statusErr serverStatusError
	if errors.As(err, &statusErr) {
		return false
	}
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

type serverStatusError struct {
	status string
}

func (e serverStatusError) Error() string {
	return fmt.Sprintf("server returned %s", e.status)
}

func gzipData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
