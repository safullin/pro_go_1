package grpcserver

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/safullin/pro_go_1/internal/audit"
	"github.com/safullin/pro_go_1/internal/model"
	metricspb "github.com/safullin/pro_go_1/internal/proto"
	"github.com/safullin/pro_go_1/internal/repository"
)

// Server реализует gRPC-сервис приёма метрик.
type Server struct {
	metricspb.UnimplementedMetricsServer
	storage repository.MetricsRepository
	auditor *audit.Publisher
}

// New создаёт gRPC-сервис поверх хранилища метрик.
func New(storage repository.MetricsRepository, auditor *audit.Publisher) *Server {
	return &Server{storage: storage, auditor: auditor}
}

// UpdateMetrics сохраняет полученный батч метрик.
func (s *Server) UpdateMetrics(ctx context.Context, request *metricspb.UpdateMetricsRequest) (*metricspb.UpdateMetricsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	metrics := make([]model.Metrics, 0, len(request.GetMetrics()))
	names := make([]string, 0, len(request.GetMetrics()))
	for _, metric := range request.GetMetrics() {
		if metric == nil || metric.GetId() == "" {
			return nil, status.Error(codes.InvalidArgument, "metric id is required")
		}
		switch metric.GetType() {
		case metricspb.Metric_GAUGE:
			value := metric.GetValue()
			metrics = append(metrics, model.Metrics{ID: metric.GetId(), MType: model.Gauge, Value: &value})
		case metricspb.Metric_COUNTER:
			delta := metric.GetDelta()
			metrics = append(metrics, model.Metrics{ID: metric.GetId(), MType: model.Counter, Delta: &delta})
		default:
			return nil, status.Error(codes.InvalidArgument, "unknown metric type")
		}
		names = append(names, metric.GetId())
	}

	if _, err := s.storage.UpdateMetrics(ctx, metrics); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update metrics: %v", err)
	}
	if s.auditor != nil && len(names) > 0 {
		s.auditor.Publish(ctx, audit.Event{
			TS:        time.Now().Unix(),
			Metrics:   names,
			IPAddress: metadataValue(ctx, "x-real-ip"),
		})
	}
	return &metricspb.UpdateMetricsResponse{}, nil
}

func metadataValue(ctx context.Context, key string) string {
	values := metadata.ValueFromIncomingContext(ctx, key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
