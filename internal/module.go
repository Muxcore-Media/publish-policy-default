package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/Muxcore-Media/core/pkg/contracts"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
	"github.com/Muxcore-Media/publish-policy-default/internal/policy"
	"github.com/Muxcore-Media/publish-policy-default/internal/server"
)

type Module struct {
	policy    *policy.Policy
	srv       *server.PolicyServer
	grpcSrv   *grpc.Server
	lis       net.Listener
	cfgMu     sync.RWMutex
	filePath  string
	auditPath string
	id        string
	grpcAddr  string
}

type Config struct {
	ID       string
	GRPCAddr string
	FilePath string
}

func NewModule(cfg Config) *Module {
	if cfg.ID == "" {
		cfg.ID = "publish-policy-default"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = ":9102"
	}
	if cfg.FilePath == "" {
		cfg.FilePath = "policies.yaml"
	}
	if v := os.Getenv("PUBLISH_POLICY_FILE"); v != "" {
		cfg.FilePath = v
	}
	auditPath := strings.TrimSpace(os.Getenv("PUBLISH_POLICY_AUDIT_PATH"))
	return &Module{
		id:        cfg.ID,
		grpcAddr:  cfg.GRPCAddr,
		filePath:  cfg.FilePath,
		auditPath: auditPath,
	}
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID:           m.id,
		Name:         "Publish Policy Default",
		Version:      "0.2.1",
		Roles:        []string{"security"},
		Description:  "Event publish policy with globs, payload checks, rate limits, and audit export",
		Author:       "MuxCore",
		Capabilities: []string{contracts.CapabilityPublishPolicy},
		Contracts: []contracts.ContractDeclaration{
			{Repo: "github.com/Muxcore-Media/core/pkg/contracts", Interface: "PublishPolicyProvider", Version: "v0.4.0"},
		},
		MinCoreVersion: "0.4.0",
		HTTPAddr:       m.grpcAddr,
	}
}

func (m *Module) Init(ctx context.Context) error {
	var err error
	m.policy, err = policy.Load(m.filePath)
	if err != nil {
		return fmt.Errorf("load policy %q: %w", m.filePath, err)
	}
	if m.auditPath != "" {
		m.policy.SetAuditPath(m.auditPath)
		slog.Info("publish-policy audit export enabled", "path", m.auditPath)
	}
	m.srv = server.New(m.policy)
	m.lis, err = net.Listen("tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", m.grpcAddr, err)
	}
	slog.Info("publish-policy initialized", "file", m.filePath)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	m.grpcSrv = grpc.NewServer()
	m.srv.RegisterWithGRPC(m.grpcSrv)
	grpc_health_v1.RegisterHealthServer(m.grpcSrv, &healthServer{})
	modulesdk.RegisterSettings(m.grpcSrv, m.id, m)

	go func() {
		slog.Info("publish-policy gRPC started", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(m.lis); err != nil {
			slog.Error("publish-policy gRPC error", "error", err)
		}
	}()

	sighupCh := make(chan os.Signal, 1)
	signal.Notify(sighupCh, syscall.SIGHUP)
	go func() {
		for range sighupCh {
			slog.Info("SIGHUP: reloading policy")
			if err := m.ReloadPolicy(); err != nil {
				slog.Error("policy reload failed", "error", err)
			}
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
	}
	slog.Info("publish-policy stopped")
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	return nil
}

type healthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (s *healthServer) Check(_ context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (s *healthServer) Watch(_ *grpc_health_v1.HealthCheckRequest, stream grpc_health_v1.Health_WatchServer) error {
	return stream.Send(&grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING})
}
