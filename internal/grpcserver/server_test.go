package grpcserver

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/safullin/pro_go_1/internal/model"
	metricspb "github.com/safullin/pro_go_1/internal/proto"
	"github.com/safullin/pro_go_1/internal/repository"
)

type failingStorage struct {
	repository.MetricsRepository
	err error
}

func (s failingStorage) UpdateMetrics(context.Context, []model.Metrics) ([]model.Metrics, error) {
	return nil, s.err
}

func TestUpdateMetrics(t *testing.T) {
	storage := repository.NewMemStorage()
	client := newTestClient(t, storage, "192.0.2.0/24")
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-real-ip", "192.0.2.10")

	_, err := client.UpdateMetrics(ctx, &metricspb.UpdateMetricsRequest{Metrics: []*metricspb.Metric{
		{Id: "Alloc", Type: metricspb.Metric_GAUGE, Value: 100.5},
		{Id: "PollCount", Type: metricspb.Metric_COUNTER, Delta: 4},
	}})
	if err != nil {
		t.Fatalf("UpdateMetrics() error: %v", err)
	}
	if value, ok := storage.GetGauge(context.Background(), "Alloc"); !ok || value != 100.5 {
		t.Fatalf("gauge = %v, %v, want 100.5, true", value, ok)
	}
	if value, ok := storage.GetCounter(context.Background(), "PollCount"); !ok || value != 4 {
		t.Fatalf("counter = %v, %v, want 4, true", value, ok)
	}
}

func TestUpdateMetricsChecksTrustedSubnet(t *testing.T) {
	storage := repository.NewMemStorage()
	client := newTestClient(t, storage, "192.0.2.0/24")
	ctx := metadata.AppendToOutgoingContext(context.Background(), "x-real-ip", "198.51.100.10")

	_, err := client.UpdateMetrics(ctx, &metricspb.UpdateMetricsRequest{Metrics: []*metricspb.Metric{
		{Id: "Alloc", Type: metricspb.Metric_GAUGE, Value: 100.5},
	}})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("status = %s, want %s", status.Code(err), codes.PermissionDenied)
	}
	if _, ok := storage.GetGauge(context.Background(), "Alloc"); ok {
		t.Fatal("forbidden metric was saved")
	}
}

func TestUpdateMetricsAllowsEmptySubnet(t *testing.T) {
	storage := repository.NewMemStorage()
	client := newTestClient(t, storage, "")
	_, err := client.UpdateMetrics(context.Background(), &metricspb.UpdateMetricsRequest{})
	if err != nil {
		t.Fatalf("UpdateMetrics() error: %v", err)
	}
}

func TestUpdateMetricsIncludesStorageError(t *testing.T) {
	storageErr := errors.New("database connection lost")
	service := New(failingStorage{
		MetricsRepository: repository.NewMemStorage(),
		err:               storageErr,
	}, nil)

	_, err := service.UpdateMetrics(context.Background(), &metricspb.UpdateMetricsRequest{Metrics: []*metricspb.Metric{
		{Id: "Alloc", Type: metricspb.Metric_GAUGE, Value: 100.5},
	}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("status = %s, want %s", status.Code(err), codes.Internal)
	}
	if !strings.Contains(err.Error(), storageErr.Error()) {
		t.Fatalf("error = %q, want storage error", err)
	}
}

func newTestClient(t *testing.T, storage repository.MetricsRepository, cidr string) metricspb.MetricsClient {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	interceptor, err := TrustedSubnetInterceptor(cidr)
	if err != nil {
		t.Fatalf("TrustedSubnetInterceptor() error: %v", err)
	}
	server := grpc.NewServer(grpc.UnaryInterceptor(interceptor))
	metricspb.RegisterMetricsServer(server, New(storage, nil))
	go func() {
		_ = server.Serve(listener)
	}()

	connection, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		server.Stop()
		_ = listener.Close()
		t.Fatalf("grpc.NewClient() error: %v", err)
	}
	t.Cleanup(func() {
		_ = connection.Close()
		server.Stop()
		_ = listener.Close()
	})
	return metricspb.NewMetricsClient(connection)
}
