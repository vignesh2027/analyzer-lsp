package grpc

import (
	"context"
	"net"
	"testing"

	"github.com/go-logr/logr"
	"github.com/konveyor/analyzer-lsp/provider"
	pb "github.com/konveyor/analyzer-lsp/provider/internal/grpc"
	"github.com/swaggest/openapi-go/openapi3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type capabilitiesServer struct {
	pb.UnimplementedProviderServiceServer
	resp *pb.CapabilitiesResponse
}

func (s *capabilitiesServer) Capabilities(context.Context, *emptypb.Empty) (*pb.CapabilitiesResponse, error) {
	return s.resp, nil
}

func startCapabilitiesServer(t *testing.T, resp *pb.CapabilitiesResponse) pb.ProviderServiceClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pb.RegisterProviderServiceServer(srv, &capabilitiesServer{resp: resp})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewProviderServiceClient(conn)
}

func TestCapabilitiesDecodesSchemas(t *testing.T) {
	in := openapi3.SchemaOrRef{Schema: &openapi3.Schema{
		Type: &provider.SchemaTypeObject,
		Properties: map[string]openapi3.SchemaOrRef{
			"pattern": {Schema: &openapi3.Schema{Type: &provider.SchemaTypeString}},
		},
	}}
	inStruct, err := provider.SchemaToStruct(in)
	if err != nil {
		t.Fatal(err)
	}

	client := startCapabilitiesServer(t, &pb.CapabilitiesResponse{Capabilities: []*pb.Capability{
		{Name: "referenced", Input: inStruct},
		{Name: "dependency"},
	}})
	g := &grpcProvider{Client: client, log: logr.Discard()}

	caps := g.Capabilities()
	if len(caps) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(caps))
	}
	if caps[0].Input.Schema == nil {
		t.Fatal("input schema was dropped on the way through gRPC")
	}
	if _, ok := caps[0].Input.Schema.Properties["pattern"]; !ok {
		t.Errorf("pattern property missing from decoded schema: %+v", caps[0].Input.Schema.Properties)
	}
	if caps[0].Output.Schema != nil {
		t.Errorf("output was never sent, got %+v", caps[0].Output.Schema)
	}
	if caps[1].Name != "dependency" || caps[1].Input.Schema != nil {
		t.Errorf("capability without schemas decoded wrongly: %+v", caps[1])
	}
}
