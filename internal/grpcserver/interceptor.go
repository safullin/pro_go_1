package grpcserver

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/safullin/pro_go_1/internal/trustedsubnet"
)

// TrustedSubnetInterceptor проверяет IP-адрес из метаданных gRPC-запроса.
func TrustedSubnetInterceptor(cidr string) (grpc.UnaryServerInterceptor, error) {
	filter, err := trustedsubnet.New(cidr)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !filter.Allows(metadataValue(ctx, "x-real-ip")) {
			return nil, status.Error(codes.PermissionDenied, "IP address is outside trusted subnet")
		}
		return handler(ctx, request)
	}, nil
}
