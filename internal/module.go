package internal

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	"github.com/Muxcore-Media/publish-policy-default/internal/policy"
	"github.com/Muxcore-Media/publish-policy-default/internal/server"
)

type Module struct {
	policy   *policy.Policy
	srv      *server.PolicyServer
	grpcSrv  *grpc.Server
	lis      net.Listener
	filePath string
	allowAll bool
	id       string
	grpcAddr string
}

type Config struct {
	ID       string
	GRPCAddr string
	FilePath string
	AllowAll bool
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
	return &Module{
		id:       cfg.ID,
		grpcAddr: cfg.GRPCAddr,
		filePath: cfg.FilePath,
		allowAll: cfg.AllowAll,
	}
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID:           m.id,
		Name:         "Publish Policy Default",
		Version:      "0.1.0",
		Roles:        []string{"security"},
		Description:  "Default event publication access control with glob-pattern allow-list",
		Author:       "MuxCore",
		Capabilities: []string{contracts.CapabilityPublishPolicy},
		HTTPAddr:     m.grpcAddr,
	}
}

func (m *Module) Init(ctx context.Context) error {
	var err error
	if m.allowAll {
		m.policy, err = policy.Parse([]byte("\n- caller: \"*\"\n  events: [\"*\"]\n"))
	} else {
		m.policy, err = policy.Load(m.filePath)
	}
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}
	m.srv = server.New(m.policy)
	m.lis, err = net.Listen("tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", m.grpcAddr, err)
	}
	slog.Info("publish-policy initialized", "file", m.filePath, "allow_all", m.allowAll)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	m.grpcSrv = grpc.NewServer()
	m.srv.RegisterWithGRPC(m.grpcSrv)

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
			newP, err := policy.Load(m.filePath)
			if err != nil {
				slog.Error("policy reload failed", "error", err)
				continue
			}
			m.policy.ReplaceRules(newP)
			slog.Info("policy reloaded")
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
