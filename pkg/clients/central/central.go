package central

import (
	"context"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"github.com/stackrox/rox/pkg/clientconn"
	"github.com/stackrox/rox/pkg/grpc/client/authn/tokenbased"
	"github.com/stackrox/rox/pkg/mtls"
	"github.com/stackrox/rox/pkg/netutil"
	"google.golang.org/grpc"
)

// ErrNewClient represents an error to create a new central client.
const (
	ErrNewClient   = "cannot create central client"
	ErrCloseClient = "cannot close central client"
)

type grpcConfig struct {
	opts     clientconn.Options
	endpoint string
}

// NewGRPC creates a grpc connection to Central with the correct auth.
func NewGRPC(ctx context.Context, endpoint string, apiToken string) (*grpc.ClientConn, error) {
	serverName, _, _, err := netutil.ParseEndpoint(endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse endpoint")
	}
	opts := clientconn.Options{
		TLS: clientconn.TLSConfigOptions{
			ServerName: serverName,
		},
		PerRPCCreds: tokenbased.PerRPCCredentials(apiToken),
	}
	return createGRPCConn(ctx, grpcConfig{
		opts:     opts,
		endpoint: endpoint,
	})
}

func createGRPCConn(ctx context.Context, c grpcConfig) (*grpc.ClientConn, error) {
	const initialBackoffDuration = 100 * time.Millisecond
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(initialBackoffDuration)),
		grpc_retry.WithMax(3),
	}

	grpcDialOpts := []grpc.DialOption{
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpts...)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpts...)),
	}

	connection, err := clientconn.GRPCConnection(ctx, mtls.CentralSubject, c.endpoint, c.opts, grpcDialOpts...)
	return connection, errors.WithStack(err)
}
