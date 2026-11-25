package plugin

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Marmot's server runs data queries on an InstanceManager, so if the manager could act, a type assertion on the server's query runner would reach a local plugin's actions. Actions live on ActionHost instead, and this keeps them off the manager.
func TestInstanceManagerCannotAct(t *testing.T) {
	typ := reflect.TypeOf(&InstanceManager{})
	for i := range typ.NumMethod() {
		assert.NotContains(t, typ.Method(i).Name, "Action", "InstanceManager is the server's query runner; actions belong on ActionHost only")
	}
}

// Actions only ever run inside a customer's serving process, never in Marmot's server, so the only code that may build an ActionHost is the serving command's file. The rest of internal/cmd is not exempt, since Marmot's server command lives there too. Every other package reaches actions, if at all, through pluginserve's Hub, which only talks to served sessions.
func TestOnlyTheServingProcessHostsActions(t *testing.T) {
	root := moduleRoot(t)
	allowed := filepath.Join(root, "internal", "cmd", "serve.go")
	self := filepath.Join(root, "internal", "plugin")

	var offenders []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			// Nested modules, such as the plugins, are built on their own and cannot reach this package's internals.
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" || name == "web" || hasGoMod(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if path == allowed || filepath.Dir(path) == self {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && (sel.Sel.Name == "NewActionHost" || sel.Sel.Name == "ActionHost") {
				rel, _ := filepath.Rel(root, path)
				offenders = append(offenders, rel)
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, offenders, "only internal/cmd/serve.go, for marmot ingest --serve, may host plugin actions")
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for !hasGoMod(dir) {
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "no go.mod above the test")
		dir = parent
	}
	return dir
}

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}

var (
	fakeActorOnce sync.Once
	fakeActorErr  error
)

// registerFakeActor builds and registers the fake actor once per test binary, as registerFakeQuerier does.
func registerFakeActor(t *testing.T) {
	t.Helper()
	fakeActorOnce.Do(func() {
		dir, err := os.MkdirTemp("", "marmot-fakeactor")
		if err != nil {
			fakeActorErr = err
			return
		}
		path := filepath.Join(dir, "fakeactor")
		cmd := exec.Command("go", "build", "-o", path, ".")
		cmd.Dir = "testdata/fakeactor"
		if out, err := cmd.CombinedOutput(); err != nil {
			fakeActorErr = fmt.Errorf("building fake actor: %v\n%s", err, out)
			return
		}
		if _, err := GetRegistry().Get("fakeactor"); err == nil {
			return
		}
		_, fakeActorErr = registerExternalPlugin(path)
	})
	require.NoError(t, fakeActorErr)
}

func TestActionHostGatesOnTheProcessManifest(t *testing.T) {
	registerFakeActor(t)
	manager := NewInstanceManager(time.Minute)
	t.Cleanup(manager.Stop)
	host := NewActionHost(manager)
	ctx := context.Background()

	state := filepath.Join(t.TempDir(), "count")
	config := map[string]any{"state_file": state}
	req := pluginsdk.ActionRequest{Action: "set_count", Params: map[string]any{"count": 4}, Identity: "user:alice", Origin: "task_run:1", Precondition: "v1"}

	specs, err := host.ActionManifest(ctx, "fakeactor")
	require.NoError(t, err)
	require.Len(t, specs, 1)

	var gated []pluginsdk.ActionSpec
	refuse := errors.New("refused by the gate")
	_, err = host.ExecuteAction(ctx, "fakeactor", config, req, func(s []pluginsdk.ActionSpec) error { gated = s; return refuse })
	assert.ErrorIs(t, err, refuse, "the gate's error comes back as it was")
	assert.Equal(t, pluginsdk.ManifestDigest(specs), pluginsdk.ManifestDigest(gated), "the gate sees the process's own manifest")
	assert.NoFileExists(t, state, "a refused action never reaches the plugin")

	allow := func([]pluginsdk.ActionSpec) error { return nil }
	plan, err := host.PlanAction(ctx, "fakeactor", config, req, allow)
	require.NoError(t, err)
	assert.Equal(t, "v1", plan.Precondition)
	assert.NoFileExists(t, state, "a plan changes nothing")

	result, err := host.ExecuteAction(ctx, "fakeactor", config, req, allow)
	require.NoError(t, err)
	assert.Equal(t, "set to 4", result.Summary)
	data, err := os.ReadFile(state)
	require.NoError(t, err)
	assert.Equal(t, "4", string(data))
}

func TestActionHostRefusesPluginsWithoutActions(t *testing.T) {
	registerFakeQuerier(t)
	manager := NewInstanceManager(time.Minute)
	t.Cleanup(manager.Stop)
	_, err := NewActionHost(manager).ActionManifest(context.Background(), "fakequerier")
	assert.ErrorIs(t, err, ErrNoActions)
}
