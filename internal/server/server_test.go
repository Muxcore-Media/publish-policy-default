package server

import (
	"context"
	"testing"

	policyv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/policy/v1"
	"github.com/Muxcore-Media/publish-policy-default/internal/policy"
)

func newTestServer(t *testing.T, yamlData string) *PolicyServer {
	t.Helper()
	p, err := policy.Parse([]byte(yamlData))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return New(p)
}

func TestAllowPublish_Allowed(t *testing.T) {
	srv := newTestServer(t, `
- caller: "mod-a"
  event_types: ["test.*"]
`)
	resp, err := srv.AllowPublish(context.Background(), &policyv1.AllowPublishRequest{
		CallerModuleId: "mod-a",
		EventType:      "test.completed",
	})
	if err != nil {
		t.Fatalf("AllowPublish: %v", err)
	}
	if !resp.Allowed {
		t.Fatal("expected publish to be allowed")
	}
}

func TestAllowPublish_Denied(t *testing.T) {
	srv := newTestServer(t, `
- caller: "mod-a"
  event_types: ["test.*"]
`)
	resp, err := srv.AllowPublish(context.Background(), &policyv1.AllowPublishRequest{
		CallerModuleId: "mod-a",
		EventType:      "other.event",
	})
	if err != nil {
		t.Fatalf("AllowPublish: %v", err)
	}
	if resp.Allowed {
		t.Fatal("expected publish to be denied")
	}
	if resp.Reason == "" {
		t.Fatal("expected non-empty reason")
	}
}

func TestAllowPublish_EmptyFields(t *testing.T) {
	srv := newTestServer(t, `
- caller: "*"
  event_types: ["*"]
`)
	_, err := srv.AllowPublish(context.Background(), &policyv1.AllowPublishRequest{
		CallerModuleId: "",
		EventType:      "test",
	})
	if err == nil {
		t.Fatal("expected error for empty caller")
	}
}

func TestAllowPublish_WildcardCatchAll(t *testing.T) {
	srv := newTestServer(t, `
- caller: "*"
  event_types: ["*"]
`)
	resp, err := srv.AllowPublish(context.Background(), &policyv1.AllowPublishRequest{
		CallerModuleId: "any",
		EventType:      "any.event.type",
	})
	if err != nil {
		t.Fatalf("AllowPublish: %v", err)
	}
	if !resp.Allowed {
		t.Fatal("expected catch-all to allow any publish")
	}
}

func TestAllowCall_DefaultsDenied(t *testing.T) {
	srv := newTestServer(t, `
- caller: "*"
  event_types: ["*"]
`)
	resp, err := srv.AllowCall(context.Background(), &policyv1.AllowCallRequest{
		CallerModuleId: "a",
		TargetModuleId: "b",
		Method:         "Get",
	})
	if err != nil {
		t.Fatalf("AllowCall: %v", err)
	}
	if resp.Allowed {
		t.Fatal("expected publish-policy-default to deny call requests")
	}
}
