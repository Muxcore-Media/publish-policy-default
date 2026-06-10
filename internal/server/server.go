package server

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Muxcore-Media/publish-policy-default/internal/policy"
	policyv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/policy/v1"
)

// PolicyServer implements the PolicyService gRPC server for publish policy enforcement.
type PolicyServer struct {
	policyv1.UnimplementedPolicyServiceServer
	policy *policy.Policy
}

// New creates a PolicyServer backed by the given policy.
func New(p *policy.Policy) *PolicyServer {
	return &PolicyServer{policy: p}
}

// RegisterWithGRPC registers the policy server with a gRPC server.
func (s *PolicyServer) RegisterWithGRPC(srv *grpc.Server) {
	policyv1.RegisterPolicyServiceServer(srv, s)
}

// AllowCall is not implemented by publish-policy-default — call policy is handled
// by call-policy-default. Returns denied.
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

// AllowPublish checks whether a caller module may publish an event of the given type.
func (s *PolicyServer) AllowPublish(ctx context.Context, req *policyv1.AllowPublishRequest) (*policyv1.AllowPublishResponse, error) {
	caller := req.GetCallerModuleId()
	eventType := req.GetEventType()

	if caller == "" || eventType == "" {
		return nil, status.Error(codes.InvalidArgument, "caller_module_id and event_type are required")
	}

	allowed, reason := s.policy.Allow(caller, eventType)
	if !allowed {
		slog.Warn("publish policy: denied",
			"caller", caller,
			"event_type", eventType,
			"reason", reason,
		)
		return &policyv1.AllowPublishResponse{Allowed: false, Reason: reason}, nil
	}

	return &policyv1.AllowPublishResponse{Allowed: true}, nil
}
