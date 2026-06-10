package server

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Muxcore-Media/publish-policy-default/internal/policy"
	policyv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/policy/v1"
)

// PolicyServer implements the PolicyService gRPC server for publish policy enforcement.
type PolicyServer struct {
	policyv1.UnimplementedPolicyServiceServer
	policy      *policy.Policy
	allowed     atomic.Int64
	denied      atomic.Int64
}

// New creates a PolicyServer backed by the given policy.
func New(p *policy.Policy) *PolicyServer {
	return &PolicyServer{policy: p}
}

// RegisterWithGRPC registers the policy server with a gRPC server.
func (s *PolicyServer) RegisterWithGRPC(srv *grpc.Server) {
	policyv1.RegisterPolicyServiceServer(srv, s)
}

// Metrics returns Prometheus-format metrics text.
func (s *PolicyServer) Metrics() string {
	var b strings.Builder
	b.WriteString("# HELP publish_policy_allowed_total Total event publishes allowed by policy\n")
	b.WriteString("# TYPE publish_policy_allowed_total counter\n")
	fmt.Fprintf(&b, "publish_policy_allowed_total %d\n", s.allowed.Load())
	b.WriteString("# HELP publish_policy_denied_total Total event publishes denied by policy\n")
	b.WriteString("# TYPE publish_policy_denied_total counter\n")
	fmt.Fprintf(&b, "publish_policy_denied_total %d\n", s.denied.Load())
	return b.String()
}

func (s *PolicyServer) AllowCall(ctx context.Context, req *policyv1.AllowCallRequest) (*policyv1.AllowCallResponse, error) {
	slog.Warn("call policy: not implemented by publish-policy-default, denying",
		"caller", req.GetCallerModuleId(),
		"target", req.GetTargetModuleId(),
	)
	return &policyv1.AllowCallResponse{
		Allowed: false,
		Reason:  "call policy is not handled by this module — deploy call-policy-default",
	}, nil
}

func (s *PolicyServer) AllowPublish(ctx context.Context, req *policyv1.AllowPublishRequest) (*policyv1.AllowPublishResponse, error) {
	caller := req.GetCallerModuleId()
	eventType := req.GetEventType()

	if caller == "" || eventType == "" {
		return nil, status.Error(codes.InvalidArgument, "caller_module_id and event_type are required")
	}

	allowed, reason := s.policy.Allow(caller, eventType)
	if !allowed {
		s.denied.Add(1)
		slog.Warn("publish policy: denied",
			"caller", caller,
			"event_type", eventType,
			"reason", reason,
		)
		return &policyv1.AllowPublishResponse{Allowed: false, Reason: reason}, nil
	}

	s.allowed.Add(1)
	return &policyv1.AllowPublishResponse{Allowed: true}, nil
}
