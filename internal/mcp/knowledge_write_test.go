package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/marmotdata/marmot/internal/core/docs"
	"github.com/marmotdata/marmot/internal/core/knowledge"
	"github.com/marmotdata/marmot/pkg/config"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubKnowledge struct{}

func (stubKnowledge) Get(context.Context, string) (*knowledge.Entry, error) { return nil, nil }
func (stubKnowledge) List(context.Context, knowledge.ListFilter) ([]knowledge.Summary, int, error) {
	return nil, 0, nil
}
func (stubKnowledge) About(context.Context, string, knowledge.AboutFilter) (*knowledge.AboutPage, error) {
	return &knowledge.AboutPage{}, nil
}
func (stubKnowledge) Save(context.Context, string, knowledge.SaveInput, string) (*knowledge.Entry, error) {
	return nil, nil
}

type stubDocs struct{}

func (stubDocs) GetPageTree(context.Context, docs.EntityType, string) (*docs.PageTree, error) {
	return &docs.PageTree{}, nil
}
func (stubDocs) CreatePage(context.Context, docs.EntityType, string, docs.CreatePageInput, *string) (*docs.Page, error) {
	return &docs.Page{}, nil
}

func listTools(t *testing.T, s *Server, p stubPrincipal) ([]string, string) {
	t.Helper()
	server := s.CreateMCPServer(context.Background(), p, "")
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "1"}, nil)
	ct, st := mcpsdk.NewInMemoryTransports()
	_, err := server.Connect(context.Background(), st, nil)
	require.NoError(t, err)
	session, err := client.Connect(context.Background(), ct, nil)
	require.NoError(t, err)
	defer session.Close()
	res, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names, session.InitializeResult().Instructions
}

func TestCreateKnowledgeOnlyOfferedToWriters(t *testing.T) {
	s := NewServer(nil, nil, nil, nil, nil, nil, nil, nil, &config.Config{}, nil, stubKnowledge{}, stubDocs{})

	writer := stubPrincipal{permissions: map[string]bool{"assets:manage": true}}
	names, instructions := listTools(t, s, writer)
	assert.Contains(t, names, "create_knowledge")
	assert.Contains(t, names, "search_knowledge")
	assert.Contains(t, instructions, "create_knowledge only after the user says yes")

	reader := stubPrincipal{permissions: map[string]bool{"assets:view": true}}
	names, instructions = listTools(t, s, reader)
	assert.NotContains(t, names, "create_knowledge")
	assert.Contains(t, names, "read_knowledge")
	assert.NotContains(t, instructions, "create_knowledge", "read-only callers must not be told they can save")
}

func TestCreateKnowledgeRefusesReadOnlyCallerAtCallTime(t *testing.T) {
	tc := &ToolContext{principal: stubPrincipal{permissions: map[string]bool{"assets:view": true}}, knowledgeService: stubKnowledge{}, docsService: stubDocs{}}
	res, _, err := tc.createKnowledge(context.Background(), nil, CreateKnowledgeInput{Kind: "runbook", Title: "x", Summary: "x", Writeup: "x"})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestPreviewIDBindsContentCallerAndTime(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	alice := &ToolContext{principal: stubPrincipal{}, previewKey: key}
	args := CreateKnowledgeInput{Kind: "runbook", Title: "Restart Flux", Summary: "Make Flux apply a push now.", Writeup: "1. Run make flux-reconcile."}

	id := alice.previewID(args, time.Now().Add(previewTTL))
	assert.True(t, alice.validPreview(args, id), "the preview's own arguments are approved")

	withID := args
	withID.PreviewID = id
	assert.True(t, alice.validPreview(withID, id), "sending the preview_id back doesn't change what was approved")

	changed := args
	changed.Writeup = "1. Run make flux-reconcile.\n2. Also delete the namespace."
	assert.False(t, alice.validPreview(changed, id), "content changed after the preview needs a new preview")

	expired := alice.previewID(args, time.Now().Add(-time.Minute))
	assert.False(t, alice.validPreview(args, expired), "an expired preview is refused")

	otherKey := &ToolContext{principal: stubPrincipal{}, previewKey: []byte("another key, as after a restart.")}
	assert.False(t, otherKey.validPreview(args, id), "a preview from before a restart is refused")

	assert.False(t, alice.validPreview(args, "garbage"))
}

func TestFindSecret(t *testing.T) {
	secrets := map[string]string{
		"aws":        "export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
		"github":     "token ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"marmot":     "X-API-Key: mrmt_u_i_xABmjqX-QIJcyZDyAAmvpiDpkSIXX",
		"private":    "-----BEGIN OPENSSH PRIVATE KEY-----",
		"password":   "password: hunter2hunter2",
		"connection": "postgres://marmot:s3cretpass@db.internal:5432/marmot",
	}
	for name, text := range secrets {
		assert.NotEmpty(t, findSecret(text), name)
	}

	clean := []string{
		"Read the password from the secret manager.",
		`printf %s "$VALUE" | gcloud secrets versions add <secret> --data-file=-`,
		"postgres://marmot:${PG_PASSWORD}@db.internal:5432/marmot",
		"password: <database-password>",
		"Set api_key to ${MARMOT_API_KEY}.",
		"kubectl -n monitoring logs deploy/alloy",
	}
	for _, text := range clean {
		assert.Empty(t, findSecret(text), text)
	}
}

func TestSimilarTitles(t *testing.T) {
	assert.True(t, similarTitles("WAL archive stuck", "Postgres WAL archive stuck"))
	assert.True(t, similarTitles("Force Flux to reconcile", "Make Flux reconcile now"))
	assert.False(t, similarTitles("Test entry please ignore", "WAL archive stuck"))
	assert.False(t, similarTitles("Update Logto's emails and sign-in page", "Test entry please ignore"))
}
