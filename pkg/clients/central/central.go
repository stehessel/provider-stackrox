package central

import (
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"github.com/stackrox/rox/pkg/clientconn"
	"github.com/stackrox/rox/pkg/grpc/client/authn/tokenbased"
	"github.com/stackrox/rox/pkg/mtls"
	"github.com/stackrox/rox/pkg/roxctl/common"
	"google.golang.org/grpc"
)

// ErrNewClient represents an error to create a new central client.
const ErrNewClient = "cannot create central client"

type grpcConfig struct {
	insecure   bool
	opts       clientconn.Options
	serverName string
	endpoint   string
}

// NewGRPC creates a grpc connection to Central with the correct auth.
func NewGRPC(serverName string, endpoint string, apiToken string) (*grpc.ClientConn, error) {
	opts := clientconn.Options{
		TLS: clientconn.TLSConfigOptions{
			ServerName:         serverName,
			InsecureSkipVerify: true,
		},
		PerRPCCreds: tokenbased.PerRPCCredentials(apiToken),
	}

	return createGRPCConn(grpcConfig{
		insecure:   true,
		opts:       opts,
		serverName: serverName,
		endpoint:   endpoint,
	})
}

func createGRPCConn(c grpcConfig) (*grpc.ClientConn, error) {
	const initialBackoffDuration = 100 * time.Millisecond
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithBackoff(grpc_retry.BackoffExponential(initialBackoffDuration)),
		grpc_retry.WithMax(3),
	}

	grpcDialOpts := []grpc.DialOption{
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpts...)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(retryOpts...)),
	}

	connection, err := clientconn.GRPCConnection(common.Context(), mtls.CentralSubject, c.endpoint, c.opts, grpcDialOpts...)
	return connection, errors.WithStack(err)
}
