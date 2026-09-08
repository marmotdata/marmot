package nifi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	pluginsdk "github.com/marmotdata/plugin-sdk"
	"github.com/marmotdata/plugin-sdk/mrn"
	"github.com/stretchr/testify/require"
)

// fakeNiFi stands in for a NiFi server. Tests describe a group tree with
// the builders below, or hand it verbatim responses, and then run a real
// discovery against it. The JSON it serves has the shape NiFi 2.11
// returns (see fixtures_test.go for captured responses).
type fakeNiFi struct {
	username string
	password string
	jwt      string
	version  string

	root     *fakeGroup
	flows    map[string]string
	groups   map[string]string
	services map[string]string
	failing  map[string]bool

	mu       sync.Mutex
	requests []fakeRequest
}

type fakeRequest struct {
	method string
	path   string
	auth   string
}

func newFakeNiFi() *fakeNiFi {
	return &fakeNiFi{
		username: "marmot",
		password: "marmotmarmot12345",
		jwt:      "eyJraWQiOiI4MDRm.fake.jwt",
		version:  "2.11.0",
		flows:    make(map[string]string),
		groups:   make(map[string]string),
		services: make(map[string]string),
		failing:  make(map[string]bool),
	}
}

// withRoot registers a group tree built with the fake builders.
func (f *fakeNiFi) withRoot(root *fakeGroup) *fakeNiFi {
	f.root = root
	return f
}

// withRawFlow serves a verbatim /flow/process-groups/{id} body.
func (f *fakeNiFi) withRawFlow(id, body string) *fakeNiFi {
	f.flows[id] = body
	return f
}

// withRawGroup serves a verbatim /process-groups/{id} body.
func (f *fakeNiFi) withRawGroup(id, body string) *fakeNiFi {
	f.groups[id] = body
	return f
}

// withRawService serves a verbatim /controller-services/{id} body.
func (f *fakeNiFi) withRawService(id, body string) *fakeNiFi {
	f.services[id] = body
	return f
}

// withService registers a controller service with the given properties.
func (f *fakeNiFi) withService(id, name, typ string, properties map[string]string, sensitive ...string) *fakeNiFi {
	props := make(map[string]any, len(properties))
	descriptors := make(map[string]any, len(properties))
	for key, value := range properties {
		props[key] = value
		descriptors[key] = map[string]any{"name": key, "sensitive": false}
	}
	for _, key := range sensitive {
		props[key] = "********"
		descriptors[key] = map[string]any{"name": key, "sensitive": true}
	}
	f.services[id] = marshal(map[string]any{
		"id": id,
		"component": map[string]any{
			"id": id, "name": name, "type": typ, "state": "ENABLED",
			"properties": props, "descriptors": descriptors,
		},
	})
	return f
}

// failingAt makes one API path answer 500.
func (f *fakeNiFi) failingAt(path string) *fakeNiFi {
	f.failing[path] = true
	return f
}

func (f *fakeNiFi) start(t *testing.T) *httptest.Server {
	t.Helper()

	if f.root != nil {
		f.root.register(f, nil)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, apiRoot)

		f.mu.Lock()
		f.requests = append(f.requests, fakeRequest{r.Method, path, r.Header.Get("Authorization")})
		f.mu.Unlock()

		if r.Method == http.MethodPost && path == "/access/token" {
			require.NoError(t, r.ParseForm())
			if r.Form.Get("username") != f.username || r.Form.Get("password") != f.password {
				http.Error(w, "The supplied username and password are not valid.", http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(f.jwt))
			return
		}

		if f.password != "" && r.Header.Get("Authorization") != "Bearer "+f.jwt {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if f.failing[path] {
			http.Error(w, "An unexpected error has occurred", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case path == "/flow/about":
			_, _ = w.Write([]byte(strings.Replace(fixtureAbout, "2.11.0", f.version, 1)))
		case strings.HasPrefix(path, "/flow/process-groups/"):
			f.serve(w, f.flows, f.resolve(strings.TrimPrefix(path, "/flow/process-groups/")), "group")
		case strings.HasPrefix(path, "/process-groups/"):
			f.serve(w, f.groups, f.resolve(strings.TrimPrefix(path, "/process-groups/")), "group")
		case strings.HasPrefix(path, "/controller-services/"):
			f.serve(w, f.services, strings.TrimPrefix(path, "/controller-services/"), "controller service")
		default:
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	}))

	t.Cleanup(server.Close)
	return server
}

// resolve maps NiFi's "root" alias to the root group's id.
func (f *fakeNiFi) resolve(id string) string {
	if id != "root" {
		return id
	}
	if f.root != nil {
		return f.root.id
	}
	return fixtureRootID
}

func (f *fakeNiFi) serve(w http.ResponseWriter, bodies map[string]string, id, kind string) {
	body, ok := bodies[id]
	if !ok {
		http.Error(w, "Unable to locate "+kind+" with id '"+id+"'.", http.StatusNotFound)
		return
	}
	_, _ = w.Write([]byte(body))
}

// paths returns every API path requested so far, in order.
func (f *fakeNiFi) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]string, 0, len(f.requests))
	for _, r := range f.requests {
		out = append(out, r.method+" "+r.path)
	}
	return out
}

// authHeaders returns the Authorization header of every request after
// the login.
func (f *fakeNiFi) authHeaders() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []string
	for _, r := range f.requests {
		if r.path != "/access/token" {
			out = append(out, r.auth)
		}
	}
	return out
}

// fakeGroup is one process group under construction.
type fakeGroup struct {
	id       string
	name     string
	comments string
	parent   *fakeGroup
	children []*fakeGroup

	processors  []*fakeProcessor
	inputPorts  []fakePort
	outputPorts []fakePort
	connections []fakeConnection

	running, stopped, invalid, disabled int
}

type fakePort struct {
	id, name, typ string
}

type fakeConnection struct {
	source, destination string
	relationships       []string
}

func newGroup(id, name string) *fakeGroup {
	return &fakeGroup{id: id, name: name}
}

func (g *fakeGroup) commented(comments string) *fakeGroup {
	g.comments = comments
	return g
}

func (g *fakeGroup) counts(running, stopped, invalid, disabled int) *fakeGroup {
	g.running, g.stopped, g.invalid, g.disabled = running, stopped, invalid, disabled
	return g
}

func (g *fakeGroup) child(c *fakeGroup) *fakeGroup {
	c.parent = g
	g.children = append(g.children, c)
	return g
}

func (g *fakeGroup) add(p *fakeProcessor) *fakeGroup {
	g.processors = append(g.processors, p)
	return g
}

func (g *fakeGroup) inputPort(id, name string) *fakeGroup {
	g.inputPorts = append(g.inputPorts, fakePort{id, name, "INPUT_PORT"})
	return g
}

func (g *fakeGroup) outputPort(id, name string) *fakeGroup {
	g.outputPorts = append(g.outputPorts, fakePort{id, name, "OUTPUT_PORT"})
	return g
}

// connect adds a connection listed under this group. The endpoints are
// component ids anywhere in the tree; their type and group are looked up
// when the flow is rendered, the way NiFi reports them.
func (g *fakeGroup) connect(source, destination string, relationships ...string) *fakeGroup {
	g.connections = append(g.connections, fakeConnection{source, destination, relationships})
	return g
}

// fakeProcessor is one processor under construction. The defaults are
// what NiFi gives a freshly created processor.
type fakeProcessor struct {
	id, name, typ    string
	state            string
	validation       string
	runStatus        string
	comments         string
	period, strategy string
	properties       map[string]string
	sensitive        map[string]bool
	relationships    []string
}

func processor(id, name, typ string) *fakeProcessor {
	return &fakeProcessor{
		id: id, name: name, typ: typ,
		state: "STOPPED", validation: "VALID", runStatus: "Stopped",
		period: "0 sec", strategy: "TIMER_DRIVEN",
		properties:    map[string]string{},
		sensitive:     map[string]bool{},
		relationships: []string{"success", "failure"},
	}
}

func (p *fakeProcessor) with(name, value string) *fakeProcessor {
	p.properties[name] = value
	return p
}

// secret sets a sensitive property. NiFi never returns the value, only
// asterisks, and marks the descriptor sensitive.
func (p *fakeProcessor) secret(name string) *fakeProcessor {
	p.properties[name] = "********"
	p.sensitive[name] = true
	return p
}

func (p *fakeProcessor) running() *fakeProcessor {
	p.state, p.runStatus = "RUNNING", "Running"
	return p
}

func (p *fakeProcessor) invalid() *fakeProcessor {
	p.validation, p.runStatus = "INVALID", "Invalid"
	return p
}

func (p *fakeProcessor) disabled() *fakeProcessor {
	p.state, p.runStatus = "DISABLED", "Disabled"
	return p
}

func (p *fakeProcessor) commented(comments string) *fakeProcessor {
	p.comments = comments
	return p
}

func (p *fakeProcessor) scheduled(period, strategy string) *fakeProcessor {
	p.period, p.strategy = period, strategy
	return p
}

func (p *fakeProcessor) relations(names ...string) *fakeProcessor {
	p.relationships = names
	return p
}

// register renders the group and its subtree into the fake's response
// tables.
func (g *fakeGroup) register(f *fakeNiFi, chain []*fakeGroup) {
	chain = append(chain, g)

	f.groups[g.id] = marshal(map[string]any{
		"revision":  map[string]any{"version": 0},
		"id":        g.id,
		"component": g.component(),
	})

	children := make([]any, 0, len(g.children))
	for _, c := range g.children {
		children = append(children, map[string]any{"id": c.id, "component": c.component()})
	}

	processors := make([]any, 0, len(g.processors))
	for _, p := range g.processors {
		processors = append(processors, p.render(g.id))
	}

	connections := make([]any, 0, len(g.connections))
	for i, c := range g.connections {
		var relationships any
		if c.relationships != nil {
			relationships = c.relationships
		}
		connections = append(connections, map[string]any{
			"id": g.id + "-conn-" + string(rune('a'+i)),
			"component": map[string]any{
				"id":                    g.id + "-conn-" + string(rune('a'+i)),
				"parentGroupId":         g.id,
				"source":                f.root.endpoint(c.source),
				"destination":           f.root.endpoint(c.destination),
				"selectedRelationships": relationships,
			},
		})
	}

	var breadcrumb map[string]any
	for _, link := range chain {
		next := map[string]any{"id": link.id, "breadcrumb": map[string]any{"id": link.id, "name": link.name}}
		if breadcrumb != nil {
			next["parentBreadcrumb"] = breadcrumb
		}
		breadcrumb = next
	}

	f.flows[g.id] = marshal(map[string]any{
		"processGroupFlow": map[string]any{
			"id":         g.id,
			"uri":        "https://nifi.example.com:8443/nifi-api/flow/process-groups/" + g.id,
			"breadcrumb": breadcrumb,
			"flow": map[string]any{
				"processGroups":       children,
				"remoteProcessGroups": []any{},
				"processors":          processors,
				"inputPorts":          renderPorts(g.id, g.inputPorts),
				"outputPorts":         renderPorts(g.id, g.outputPorts),
				"connections":         connections,
				"labels":              []any{},
				"funnels":             []any{},
			},
			"lastRefreshed": "02:24:29 UTC",
		},
	})

	for _, c := range g.children {
		c.register(f, chain)
	}
}

func (g *fakeGroup) component() map[string]any {
	component := map[string]any{
		"id":              g.id,
		"name":            g.name,
		"comments":        g.comments,
		"runningCount":    g.running,
		"stoppedCount":    g.stopped,
		"invalidCount":    g.invalid,
		"disabledCount":   g.disabled,
		"inputPortCount":  len(g.inputPorts),
		"outputPortCount": len(g.outputPorts),
	}
	if g.parent != nil {
		component["parentGroupId"] = g.parent.id
	}
	return component
}

// endpoint renders a connection end the way NiFi does: the component's
// own id, type and group, whichever group the connection is listed in.
func (g *fakeGroup) endpoint(id string) map[string]any {
	for _, p := range g.processors {
		if p.id == id {
			return map[string]any{"id": id, "type": "PROCESSOR", "groupId": g.id, "name": p.name, "running": p.state == "RUNNING"}
		}
	}
	for _, ports := range [][]fakePort{g.inputPorts, g.outputPorts} {
		for _, port := range ports {
			if port.id == id {
				return map[string]any{"id": id, "type": port.typ, "groupId": g.id, "name": port.name, "running": false}
			}
		}
	}
	for _, c := range g.children {
		if endpoint := c.endpoint(id); endpoint != nil {
			return endpoint
		}
	}
	return nil
}

func (p *fakeProcessor) render(groupID string) map[string]any {
	properties := make(map[string]any, len(p.properties))
	descriptors := make(map[string]any, len(p.properties))
	for name, value := range p.properties {
		properties[name] = value
		descriptors[name] = map[string]any{
			"name": name, "displayName": name, "sensitive": p.sensitive[name], "required": false,
		}
	}

	relationships := make([]any, 0, len(p.relationships))
	for _, name := range p.relationships {
		relationships = append(relationships, map[string]any{"name": name, "autoTerminate": false})
	}

	return map[string]any{
		"id":  p.id,
		"uri": "https://nifi.example.com:8443/nifi-api/processors/" + p.id,
		"component": map[string]any{
			"id":               p.id,
			"parentGroupId":    groupID,
			"name":             p.name,
			"type":             p.typ,
			"state":            p.state,
			"validationStatus": p.validation,
			"config": map[string]any{
				"schedulingPeriod":   p.period,
				"schedulingStrategy": p.strategy,
				"comments":           p.comments,
				"properties":         properties,
				"descriptors":        descriptors,
			},
			"relationships": relationships,
		},
		"status": map[string]any{"runStatus": p.runStatus},
	}
}

func renderPorts(groupID string, ports []fakePort) []any {
	out := make([]any, 0, len(ports))
	for _, port := range ports {
		out = append(out, map[string]any{
			"id": port.id,
			"component": map[string]any{
				"id": port.id, "parentGroupId": groupID, "name": port.name,
				"state": "STOPPED", "type": port.typ, "portFunction": "STANDARD",
			},
		})
	}
	return out
}

func marshal(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// discover runs a full discovery against the fake with a username and
// password login. Extra config keys override the defaults.
func discover(t *testing.T, f *fakeNiFi, overrides pluginsdk.RawConfig) *pluginsdk.DiscoveryResult {
	t.Helper()

	result, err := tryDiscover(t, f, overrides)
	require.NoError(t, err)
	return result
}

func tryDiscover(t *testing.T, f *fakeNiFi, overrides pluginsdk.RawConfig) (*pluginsdk.DiscoveryResult, error) {
	t.Helper()

	server := f.start(t)

	config := pluginsdk.RawConfig{"host": server.URL, "username": f.username, "password": f.password}
	for key, value := range overrides {
		config[key] = value
	}

	return (&Source{}).Discover(t.Context(), config)
}

// findAsset returns the asset of one type whose MRN is the one the
// server would derive from that name.
func findAsset(result *pluginsdk.DiscoveryResult, assetType, name string) *pluginsdk.Asset {
	for i, asset := range result.Assets {
		if asset.Type != assetType || asset.MRN == nil || len(asset.Providers) == 0 {
			continue
		}
		if *asset.MRN == mrn.New(assetType, asset.Providers[0], name) {
			return &result.Assets[i]
		}
	}
	return nil
}

func hasEdge(result *pluginsdk.DiscoveryResult, source, target, edgeType string) bool {
	for _, edge := range result.Lineage {
		if edge.Source == source && edge.Target == target && edge.Type == edgeType {
			return true
		}
	}
	return false
}

func assetNames(result *pluginsdk.DiscoveryResult, assetType string) []string {
	var names []string
	for _, asset := range result.Assets {
		if asset.Type == assetType && asset.Name != nil {
			names = append(names, *asset.Name)
		}
	}
	return names
}

// standardFlow is the flow the e2e tests seed into the real NiFi: an
// Ingest group feeding a Deliver group through ports, with a Kafka and an
// S3 processor in Ingest.
func standardFlow() *fakeGroup {
	ingest := newGroup("pg-ingest", "Ingest").commented("Lands raw orders in S3 and Kafka").counts(1, 2, 2, 0).
		add(processor("p-gen", "Generate Orders", "org.apache.nifi.processors.standard.GenerateFlowFile").running().relations("success").scheduled("1 hour", "TIMER_DRIVEN")).
		add(processor("p-stamp", "Stamp Attributes", "org.apache.nifi.processors.attributes.UpdateAttribute").relations("success").with("source", "orders")).
		add(processor("p-s3", "Land in S3", "org.apache.nifi.processors.aws.s3.PutS3Object").invalid().with("Bucket", "marmot-landing")).
		add(processor("p-pub", "Publish Orders", "org.apache.nifi.kafka.processors.PublishKafka").invalid().with("Topic Name", "orders-events")).
		outputPort("port-out", "to-deliver").
		connect("p-gen", "p-stamp", "success").
		connect("p-stamp", "p-s3", "success").
		connect("p-stamp", "p-pub", "success").
		connect("p-stamp", "port-out", "success")

	deliver := newGroup("pg-deliver", "Deliver").counts(0, 2, 3, 0).
		inputPort("port-in", "from-ingest").
		add(processor("p-log", "Log Orders", "org.apache.nifi.processors.standard.LogAttribute").invalid()).
		add(processor("p-con", "Consume Orders", "org.apache.nifi.kafka.processors.ConsumeKafka").invalid().with("Topics", "orders-events, orders-dlq").with("Topic Format", "names")).
		connect("port-in", "p-log").
		connect("p-con", "p-log", "success")

	return newGroup("pg-root", "NiFi Flow").
		child(ingest).
		child(deliver).
		connect("port-out", "port-in")
}

// startTLS serves the fake over HTTPS with httptest's untrusted
// certificate. The caller registers the tree first.
func (f *fakeNiFi) startTLS(t *testing.T) *httptest.Server {
	t.Helper()

	plain := f.start(t)
	handler := plain.Config.Handler
	plain.Close()

	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	return server
}
