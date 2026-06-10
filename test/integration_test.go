package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	policyv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/policy/v1"
)

func TestPublishPolicy_DefaultDeny(t *testing.T) {
	bin := buildModule(t, "publish-policy-default")
	policyFile := writePublishPolicy(t)
	addr := ":19201"

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "--grpc-addr", addr, "--policy-file", policyFile)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := policyv1.NewPolicyServiceClient(conn)

	t.Run("allow_glob", func(t *testing.T) {
		resp, err := client.AllowPublish(ctx, &policyv1.AllowPublishRequest{
			CallerModuleId: "mod-a", EventType: "download.completed",
		})
		if err != nil {
			t.Fatalf("AllowPublish: %v", err)
		}
		if !resp.Allowed {
			t.Fatal("expected allowed")
		}
	})

	t.Run("deny_no_match", func(t *testing.T) {
		resp, err := client.AllowPublish(ctx, &policyv1.AllowPublishRequest{
			CallerModuleId: "mod-a", EventType: "transcode.started",
		})
		if err != nil {
			t.Fatalf("AllowPublish: %v", err)
		}
		if resp.Allowed {
			t.Fatal("expected denied")
		}
	})

	t.Run("call_not_implemented", func(t *testing.T) {
		resp, err := client.AllowCall(ctx, &policyv1.AllowCallRequest{
			CallerModuleId: "a", TargetModuleId: "b", Method: "Get",
		})
		if err != nil {
			t.Fatalf("AllowCall: %v", err)
		}
		if resp.Allowed {
			t.Fatal("expected publish-policy to deny call requests")
		}
	})
}

func writePublishPolicy(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "policies.yaml")
	os.WriteFile(p, []byte(strings.TrimSpace(`
- caller: "mod-a"
  event_types: ["download.*", "media.*"]
- caller: "*"
  event_types: ["module.*"]
`)), 0644)
	return p
}

func buildModule(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, name)
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/module")
	cmd.Dir = findRepoRoot(t)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build: %v", err)
	}
	return bin
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	dir, _ := os.Getwd()
	for dir != "/" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("cannot find repo root")
	return ""
}

func init() {
	fmt.Fprintln(os.Stderr, "integration tests: building module binary...")
}
