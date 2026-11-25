//go:build kind_e2e

package pluginserve

// End to end against a real cluster: the serving process, the real Kubernetes plugin binary and the hub, acting on a kind cluster through the least-privilege chart. Run with:
//
//	MARMOT_KIND_PLUGIN=<built plugin> MARMOT_KIND_DIR=<dir with ca.crt, token> MARMOT_TEST_POSTGRES_DSN=... \
//	  go test -tags kind_e2e -run TestKind ./internal/pluginserve/

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/marmotdata/marmot/internal/plugin"
)

const kindContext = "kind-marmot-actions"

func kubectl(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("kubectl", append([]string{"--context", kindContext}, args...)...).CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

var kindPolicy = approvalsYAML() + `
sources:
  SOURCE:
    query: false
    manifest: DIGEST
    origins: ["task_run:*"]
    actions:
      scale:
        params:
          namespace: {values: [payments]}
          replicas: {min: 1, max: 5}
      rollout_restart: {}
      suspend_cronjob: {}
      resume_cronjob: {}
      cordon: {}
      uncordon: {}
      workload_status: {}
      pod_logs:
        params:
          tail_lines: {max: 50}
      recent_events: {}
    triggers:
      workload_unhealthy:
        params:
          kinds: [deployment]
          namespaces: [payments]
          conditions: [crash_looping, image_pull_failing]
          for_minutes: 1
        cooldown: 1h
      custom_condition:
        params:
          kind: deployment
          namespaces: [payments]
          name: unavailable
          when: object.status.?unavailableReplicas.orValue(0) > 0
          for_minutes: 1
        cooldown: 1h
`

func TestKindActionsEndToEnd(t *testing.T) {
	binary, dir := os.Getenv("MARMOT_KIND_PLUGIN"), os.Getenv("MARMOT_KIND_DIR")
	if binary == "" || dir == "" {
		t.Skip("MARMOT_KIND_PLUGIN and MARMOT_KIND_DIR not set")
	}
	ca, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	require.NoError(t, err)
	token, err := os.ReadFile(filepath.Join(dir, "token"))
	require.NoError(t, err)
	server := kubectl(t, "config", "view", "--raw", "-o", "jsonpath={.clusters[?(@.name==\""+kindContext+"\")].cluster.server}")

	plugin.LoadBinary(binary)
	host := plugin.NewActionHost(plugin.NewInstanceManager(time.Minute))
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	specs, err := host.ActionManifest(ctx, "kubernetes")
	require.NoError(t, err)
	digest := pluginsdk.ManifestDigest(specs)

	h := newHarness(t)
	hub := h.startHub()
	events := make(chan VerifiedTriggerEvent, 64)
	defer hub.SubscribeTriggers(func(_ context.Context, ev VerifiedTriggerEvent) { events <- ev })()
	source, principal := newSourceName(), servePrincipal()
	auditPath := filepath.Join(t.TempDir(), "audit.log")
	audit, err := OpenAuditLog(auditPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = audit.Close() })
	body := strings.NewReplacer("SOURCE", source, "DIGEST", digest).Replace(kindPolicy)
	policy, err := LoadPolicy(writePolicy(t, body))
	require.NoError(t, err)

	h.serveWith(h.enroller(hub), hub, principal, sessionCredential, source, &fakeQuerier{}, func(o *ServeOptions) {
		cfg := map[string]any{
			"host": server, "token": string(token), "ca_certificate": string(ca),
			"actions": map[string]any{"namespaces": []string{"payments", "checkout"}, "max_replicas": 10},
		}
		o.Sources[0].PluginID, o.Sources[0].Config, o.Sources[0].DisplayConfig = "kubernetes", cfg, map[string]any{"host": server}
		o.Actor, o.Watcher, o.Policy, o.AuditLog, o.AllowActions = host, host, policy, audit, true
	})

	call := func(action string, params map[string]any) ActionCall {
		return ActionCall{Source: source, ID: uuid.NewString(), ManifestDigest: digest,
			Request: pluginsdk.ActionRequest{Action: action, Params: params, Identity: "user:alice", Origin: "task_run:e2e"}}
	}
	replicas := func(ns, name string) string {
		return kubectl(t, "-n", ns, "get", "deploy", name, "-o", "jsonpath={.spec.replicas}")
	}
	// planned plans an action and returns it ready to execute, as a person reviewing the plan would see it
	planned := func(t *testing.T, c ActionCall) ActionCall {
		t.Helper()
		plan, receipt, err := hub.PlanAction(ctx, c)
		require.NoError(t, err)
		require.True(t, receipt.Verified(), receipt.Problem)
		require.NotEmpty(t, plan.Precondition)
		t.Logf("plan %s: %s\n%s", c.Request.Action, plan.Summary, plan.Diff)
		c.ID, c.Request.Precondition = uuid.NewString(), plan.Precondition
		return c
	}
	// run plans then executes an action with a Marmot-signed approval naming alice
	run := func(t *testing.T, c ActionCall) *pluginsdk.ActionResult {
		t.Helper()
		c = planned(t, c)
		c.Approval = approve(t, hub, c, 5*time.Minute, alice)
		result, receipt, err := hub.ExecuteAction(ctx, c)
		require.NoError(t, err)
		require.True(t, receipt.Verified(), receipt.Problem)
		require.Len(t, receipt.Approvers, 1)
		return result
	}
	read := func(t *testing.T, action string, params map[string]any) map[string]any {
		t.Helper()
		result, receipt, err := hub.ExecuteAction(ctx, call(action, params))
		require.NoError(t, err)
		require.True(t, receipt.Verified(), receipt.Problem)
		assert.True(t, result.Untrusted)
		var out map[string]any
		require.NoError(t, json.Unmarshal(result.Output, &out))
		assert.Equal(t, true, out["untrusted"])
		return out
	}

	// Triggers start with the serving process, so they are checked first, before the restart below replaces crashy's pods and resets how long it has been failing. The plugin evaluates every 30 s and the conditions must hold for a minute.
	t.Run("triggers fire for what is broken and not for what is healthy", func(t *testing.T) {
		seen := map[string]VerifiedTriggerEvent{}
		want := []string{"workload_unhealthy/crashy/crash_looping", "workload_unhealthy/badimage/image_pull_failing", "custom_condition/crashy/unavailable"}
		// A crashy restarted by an earlier run needs a few more crashes before it counts as crash looping again.
		deadline := time.After(5 * time.Minute)
		for !hasAll(seen, want) {
			select {
			case ev := <-events:
				key := ev.Trigger + "/" + ev.Target.Name + "/" + ev.Facts["condition"].(string)
				t.Logf("trigger event %s: %v", key, ev.Facts)
				assert.NotEqual(t, "api", ev.Target.Name, "healthy payments/api never fires")
				assert.Equal(t, source, ev.Source)
				seen[key] = ev
			case <-deadline:
				t.Fatalf("only these trigger events arrived: %v", keys(seen))
			}
		}
		crash, ok := seen["workload_unhealthy/crashy/crash_looping"]
		require.True(t, ok, "crashy is crash looping")
		assert.Equal(t, "kubernetes/deployment", string(crash.Target.Kind))
		assert.Equal(t, "deployment/crashy", crash.Facts["workload"])
		assert.Positive(t, crash.Facts["restart_count"])
		_, ok = seen["workload_unhealthy/badimage/image_pull_failing"]
		assert.True(t, ok, "badimage cannot pull its image")
		_, ok = seen["custom_condition/crashy/unavailable"]
		assert.True(t, ok, "the custom CEL condition fires for crashy")
	})

	t.Run("reads show crashy crash looping, with its logs redacted", func(t *testing.T) {
		ws := read(t, "workload_status", map[string]any{"kind": "deployment", "namespace": "payments", "name": "crashy"})
		pods := ws["pods"].([]any)
		require.NotEmpty(t, pods)
		pod := pods[0].(map[string]any)
		assert.Positive(t, pod["restarts"])
		containers := pod["containers"].([]any)
		c := containers[0].(map[string]any)
		t.Logf("crashy container: %v", c)
		assert.Contains(t, []any{"CrashLoopBackOff", nil}, c["waiting_reason"])
		assert.NotEmpty(t, c["last_termination_reason"])

		// The crashed container's logs are the previous one's while it waits to restart, and its own once it has stopped again; the kubelet keeps only one of them at a time.
		var text string
		for _, previous := range []bool{true, false} {
			logs := read(t, "pod_logs", map[string]any{"namespace": "payments", "pod": pod["name"], "tail_lines": 20, "previous": previous})
			assert.Contains(t, logs, "redacted")
			text += strings.Join(toStrings(logs["lines"]), "\n")
		}
		assert.Contains(t, text, "boom")

		evs := read(t, "recent_events", map[string]any{"namespace": "payments", "limit": 10})
		assert.NotNil(t, evs["events"])

		_, _, err := hub.ExecuteAction(ctx, call("pod_logs", map[string]any{"namespace": "payments", "pod": pod["name"], "tail_lines": 200}))
		assert.Equal(t, codes.PermissionDenied, status.Code(err), "the policy caps tail_lines at 50")
	})

	t.Run("a write needs a Marmot-signed approval, once", func(t *testing.T) {
		c := planned(t, call("scale", map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": 2}))
		_, _, err := hub.ExecuteAction(ctx, c)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.Contains(t, err.Error(), "carries none")
		assert.Equal(t, "1", replicas("payments", "api"))

		c.Approval = approve(t, hub, c, 5*time.Minute, alice)
		_, receipt, err := hub.ExecuteAction(ctx, c)
		require.NoError(t, err)
		assert.Equal(t, "alice", receipt.Approvers[0].Name)
		assert.Equal(t, "2", replicas("payments", "api"))

		_, _, err = hub.ExecuteAction(ctx, c)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.Contains(t, err.Error(), "already been used")
		run(t, call("scale", map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": 1}))
	})
	refused := func(t *testing.T, c ActionCall, code codes.Code, contains string) {
		t.Helper()
		_, _, err := hub.PlanAction(ctx, c)
		require.Error(t, err)
		assert.Equal(t, code, status.Code(err), err.Error())
		assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(contains))
	}

	t.Run("scale, stamped, and undone", func(t *testing.T) {
		require.Equal(t, "1", replicas("payments", "api"))
		result := run(t, call("scale", map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": 3}))
		assert.Equal(t, "3", replicas("payments", "api"))
		assert.Equal(t, "task_run:e2e", kubectl(t, "-n", "payments", "get", "deploy", "api", "-o", `jsonpath={.metadata.annotations.marmot\.io/origin}`))
		assert.Equal(t, "user:alice", kubectl(t, "-n", "payments", "get", "deploy", "api", "-o", `jsonpath={.metadata.annotations.marmot\.io/actor}`))
		require.NotNil(t, result.Undo, "scale declares an inverse")
		undo := call(result.Undo.Action, result.Undo.Params)
		run(t, undo)
		assert.Equal(t, "1", replicas("payments", "api"))
	})

	t.Run("a target that changed after its plan is refused", func(t *testing.T) {
		c := call("scale", map[string]any{"kind": "deployment", "namespace": "payments", "name": "api", "replicas": 2})
		plan, _, err := hub.PlanAction(ctx, c)
		require.NoError(t, err)
		kubectl(t, "-n", "payments", "annotate", "deploy", "api", "someone-else=touched", "--overwrite")
		c.ID, c.Request.Precondition = uuid.NewString(), plan.Precondition
		c.Approval = approve(t, hub, c, 5*time.Minute, alice)
		_, receipt, err := hub.ExecuteAction(ctx, c)
		assert.Equal(t, codes.FailedPrecondition, status.Code(err), err)
		require.True(t, receipt.Verified(), receipt.Problem)
		assert.Equal(t, "1", replicas("payments", "api"))
		kubectl(t, "-n", "payments", "annotate", "deploy", "api", "someone-else-")
	})

	t.Run("restart a crash-looping deployment", func(t *testing.T) {
		before := kubectl(t, "-n", "payments", "get", "deploy", "crashy", "-o", `jsonpath={.spec.template.metadata.annotations.kubectl\.kubernetes\.io/restartedAt}`)
		result := run(t, call("rollout_restart", map[string]any{"kind": "deployment", "namespace": "payments", "name": "crashy"}))
		after := kubectl(t, "-n", "payments", "get", "deploy", "crashy", "-o", `jsonpath={.spec.template.metadata.annotations.kubectl\.kubernetes\.io/restartedAt}`)
		assert.NotEqual(t, before, after)
		assert.Nil(t, result.Undo, "a restart is self-healing, not undoable")
	})

	t.Run("suspend and resume a cronjob", func(t *testing.T) {
		result := run(t, call("suspend_cronjob", map[string]any{"namespace": "payments", "name": "nightly"}))
		assert.Equal(t, "true", kubectl(t, "-n", "payments", "get", "cronjob", "nightly", "-o", "jsonpath={.spec.suspend}"))
		require.NotNil(t, result.Undo)
		run(t, call(result.Undo.Action, result.Undo.Params))
		assert.Equal(t, "false", kubectl(t, "-n", "payments", "get", "cronjob", "nightly", "-o", "jsonpath={.spec.suspend}"))
	})

	t.Run("cordon, and never the last schedulable node", func(t *testing.T) {
		result := run(t, call("cordon", map[string]any{"name": "marmot-actions-worker2"}))
		assert.Equal(t, "true", kubectl(t, "get", "node", "marmot-actions-worker2", "-o", "jsonpath={.spec.unschedulable}"))
		_, _, err := hub.PlanAction(ctx, call("cordon", map[string]any{"name": "marmot-actions-worker"}))
		require.Error(t, err, "worker is now the last schedulable node")
		t.Logf("last node refusal: %v", err)
		run(t, call(result.Undo.Action, result.Undo.Params))
		assert.Empty(t, kubectl(t, "get", "node", "marmot-actions-worker2", "-o", "jsonpath={.spec.unschedulable}"))
	})

	t.Run("refusals", func(t *testing.T) {
		scale := func(ns, name string, n int) ActionCall {
			return call("scale", map[string]any{"kind": "deployment", "namespace": ns, "name": name, "replicas": n})
		}
		// The runner's policy, before the plugin is asked anything
		refused(t, scale("checkout", "web", 2), codes.PermissionDenied, "policy")
		refused(t, scale("payments", "api", 8), codes.PermissionDenied, "policy")
		refused(t, call("rollout_undo", map[string]any{"namespace": "payments", "name": "api"}), codes.PermissionDenied, "policy")
		person := call("rollout_restart", map[string]any{"kind": "deployment", "namespace": "payments", "name": "api"})
		person.Request.Origin = "console"
		refused(t, person, codes.PermissionDenied, "policy")
		stale := scale("payments", "api", 2)
		stale.ManifestDigest = "sha256:" + strings.Repeat("0", 64)
		refused(t, stale, codes.PermissionDenied, "manifest")
		// The plugin's own scope: Flux owns ledger, and internal is not an actions namespace
		_, _, err := hub.PlanAction(ctx, call("rollout_restart", map[string]any{"kind": "deployment", "namespace": "payments", "name": "ledger"}))
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "git")
		_, _, err = hub.PlanAction(ctx, call("rollout_restart", map[string]any{"kind": "deployment", "namespace": "internal", "name": "admin"}))
		require.Error(t, err)
		t.Logf("internal namespace refusal: %v", err)
		assert.Equal(t, "1", replicas("payments", "ledger"))
		assert.Equal(t, "1", replicas("internal", "admin"))
	})

	n, err := VerifyAuditLog(auditPath)
	require.NoError(t, err)
	t.Logf("audit log: %d lines, chain intact", n)
}

func keys(m map[string]VerifiedTriggerEvent) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func toStrings(v any) []string {
	items, _ := v.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, _ := item.(string)
		out = append(out, s)
	}
	return out
}

func hasAll(m map[string]VerifiedTriggerEvent, keys []string) bool {
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}
