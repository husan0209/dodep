package telemetry

import (
	"context"
	"io"
	"net"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	pb "github.com/opus-casino/proto/gen/go/bonus/v1"
)

// netListen starts a loopback listener for the gRPC test server.
func netListen() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}

// insecureCreds: mTLS is handled by the mesh (Istio), so tests use plaintext.
func insecureCreds() credentials.TransportCredentials { return insecure.NewCredentials() }

// httpGet fetches a URL and returns the body as a string.
func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	return string(raw), err
}

// stubBonusServer is a minimal BonusService implementation whose only purpose
// is to exercise the interceptor chain: GetBonus rejects an empty bonus_id and
// GetPromotions panics.
type stubBonusServer struct {
	pb.UnimplementedBonusServiceServer
}

func (stubBonusServer) GetBonus(_ context.Context, req *pb.GetBonusRequest) (*pb.GetBonusResponse, error) {
	if req.GetBonusId() == "" {
		return nil, status.Error(codes.InvalidArgument, "bonus_id is required")
	}
	return &pb.GetBonusResponse{}, nil
}

func (stubBonusServer) GetPromotions(context.Context, *pb.GetPromotionsRequest) (*pb.GetPromotionsResponse, error) {
	panic("boom")
}
