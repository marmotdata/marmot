package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
	"github.com/marmotdata/marmot/internal/core/auth"
	"github.com/marmotdata/marmot/internal/core/user"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
)

// stubPrincipal is a minimal auth.Principal for exercising the write tools' permission gate without a real auth stack.
type stubPrincipal struct {
	admin       bool
	permissions map[string]bool
}

func (p stubPrincipal) ID() string               { return "stub" }
func (p stubPrincipal) Type() auth.PrincipalType { return auth.PrincipalTypeUser }
func (p stubPrincipal) DisplayName() string      { return "stub" }
func (p stubPrincipal) AuditSubject() string     { return "user:stub" }
func (p stubPrincipal) Roles() []string          { return nil }
func (p stubPrincipal) Permissions() []string    { return nil }
func (p stubPrincipal) IsAdmin() bool            { return p.admin }
func (p stubPrincipal) AsUser() *user.User       { return nil }
func (p stubPrincipal) HasPermission(resourceType, action string) bool {
	return p.permissions[resourceType+":"+action]
}

func TestRequireManage_RefusesWithoutPermission(t *testing.T) {
	tc := &ToolContext{principal: stubPrincipal{permissions: map[string]bool{"assets:view": true}}}
	denied := tc.requireManage("assets")
	if assert.NotNil(t, denied, "a view-only principal must be refused") {
		assert.True(t, denied.IsError)
	}
}

func TestRequireManage_AllowsWithPermission(t *testing.T) {
	tc := &ToolContext{principal: stubPrincipal{permissions: map[string]bool{"assets:manage": true}}}
	assert.Nil(t, tc.requireManage("assets"), "a principal with assets:manage must be allowed")
}

func TestRequireManage_AllowsAdmin(t *testing.T) {
	tc := &ToolContext{principal: stubPrincipal{admin: true}}
	assert.Nil(t, tc.requireManage("assets"), "an admin must be allowed")
}

func TestRequireManage_RefusesNilPrincipal(t *testing.T) {
	tc := &ToolContext{}
	assert.NotNil(t, tc.requireManage("assets"), "a missing principal must be refused")
}

func TestProposalMakesNoClaimOfChange(t *testing.T) {
	res := proposal("Set docs", "diff", `{"confirm": true}`)
	assert.False(t, res.IsError)
	text := res.Content[0].(*mcpsdk.TextContent).Text
	assert.Contains(t, text, "Nothing has changed yet")
	assert.Contains(t, text, `"confirm": true`)
}

func TestWriteHelpers(t *testing.T) {
	assert.Equal(t, "_(none)_", orNone("  "))
	assert.Equal(t, "hello", orNone("hello"))
	assert.Equal(t, "Add", capitalise("add"))
	assert.Equal(t, "from", preposition("remove"))
	assert.Equal(t, "to", preposition("add"))
	assert.True(t, strings.HasPrefix(capitalise("remove"), "R"))
}

// fakeAssetService embeds the interface so only the methods the write tools touch need implementing; anything else panics, which is exactly what a test wants.
type fakeAssetService struct {
	asset.Service
	assets      map[string]*asset.Asset
	updated     *asset.UpdateInput
	addedTags   []string
	removedTags []string
	failAddTag  map[string]error
}

func (f *fakeAssetService) Get(ctx context.Context, id string) (*asset.Asset, error) {
	if a, ok := f.assets[id]; ok {
		return a, nil
	}
	return nil, asset.ErrAssetNotFound
}

func (f *fakeAssetService) Update(ctx context.Context, id string, input asset.UpdateInput) (*asset.Asset, error) {
	f.updated = &input
	return f.assets[id], nil
}

func (f *fakeAssetService) AddTag(ctx context.Context, id string, tag string) (*asset.Asset, error) {
	if err := f.failAddTag[tag]; err != nil {
		return nil, err
	}
	f.addedTags = append(f.addedTags, tag)
	return f.assets[id], nil
}

func (f *fakeAssetService) RemoveTag(ctx context.Context, id string, tag string) (*asset.Asset, error) {
	f.removedTags = append(f.removedTags, tag)
	return f.assets[id], nil
}

type fakeTeamService struct {
	TeamService
	owners  []Owner
	added   []string
	removed []string
}

func (f *fakeTeamService) ListAssetOwners(ctx context.Context, assetID string) ([]Owner, error) {
	return f.owners, nil
}

func (f *fakeTeamService) AddAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error {
	f.added = append(f.added, ownerType+":"+ownerID)
	return nil
}

func (f *fakeTeamService) RemoveAssetOwner(ctx context.Context, assetID, ownerType, ownerID string) error {
	f.removed = append(f.removed, ownerType+":"+ownerID)
	return nil
}

func writeTestAsset() *asset.Asset {
	return &asset.Asset{ID: "a1", Name: strPtr("orders"), Type: "Table", Tags: []string{"pii"}, UserDescription: strPtr("old docs")}
}

func writeToolContext(fa *fakeAssetService, ft *fakeTeamService) *ToolContext {
	return &ToolContext{assetService: fa, teamService: ft, principal: stubPrincipal{admin: true}}
}

func resultText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()
	if !assert.NotEmpty(t, res.Content) {
		return ""
	}
	return res.Content[0].(*mcpsdk.TextContent).Text
}

func TestUpdateDocumentation_ProposeThenApply(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{"a1": writeTestAsset()}}
	tc := writeToolContext(fa, &fakeTeamService{})

	res, _, err := tc.updateDocumentation(context.Background(), nil, UpdateDocumentationInput{ID: "a1", Documentation: "new docs"})
	assert.NoError(t, err)
	text := resultText(t, res)
	assert.Contains(t, text, "Proposed change")
	assert.Contains(t, text, "new docs")
	assert.Nil(t, fa.updated, "a preview must not write")

	res, _, err = tc.updateDocumentation(context.Background(), nil, UpdateDocumentationInput{ID: "a1", Documentation: "new docs", Confirm: true})
	assert.NoError(t, err)
	assert.Contains(t, resultText(t, res), "Change applied")
	if assert.NotNil(t, fa.updated) {
		assert.Equal(t, "new docs", *fa.updated.UserDescription)
	}
}

func TestUpdateDocumentation_NothingToChange(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{"a1": writeTestAsset()}}
	tc := writeToolContext(fa, &fakeTeamService{})

	res, _, _ := tc.updateDocumentation(context.Background(), nil, UpdateDocumentationInput{ID: "a1", Documentation: "old docs"})
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "Nothing to change")
	assert.Nil(t, fa.updated)
}

func TestManageTags_SkipsAlreadyCorrectTags(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{"a1": writeTestAsset()}}
	tc := writeToolContext(fa, &fakeTeamService{})

	res, _, _ := tc.manageTags(context.Background(), nil, ManageTagsInput{ID: "a1", AddTags: []string{"pii", "gold"}, RemoveTags: []string{"absent"}, Confirm: true})
	assert.Contains(t, resultText(t, res), "Change applied")
	assert.Equal(t, []string{"gold"}, fa.addedTags, "an existing tag must not be re-added")
	assert.Empty(t, fa.removedTags, "an absent tag must not be removed")
}

func TestManageTags_PartialFailureReportsApplied(t *testing.T) {
	fa := &fakeAssetService{
		assets:     map[string]*asset.Asset{"a1": writeTestAsset()},
		failAddTag: map[string]error{"two": errors.New("boom")},
	}
	tc := writeToolContext(fa, &fakeTeamService{})

	res, _, _ := tc.manageTags(context.Background(), nil, ManageTagsInput{ID: "a1", AddTags: []string{"one", "two"}, Confirm: true})
	assert.True(t, res.IsError)
	text := resultText(t, res)
	assert.Contains(t, text, `added "one"`)
	assert.Contains(t, text, "partially updated")
}

func TestManageTags_EscapesTagsInPreview(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{"a1": writeTestAsset()}}
	tc := writeToolContext(fa, &fakeTeamService{})

	res, _, _ := tc.manageTags(context.Background(), nil, ManageTagsInput{ID: "a1", AddTags: []string{"evil`tag"}})
	assert.Contains(t, resultText(t, res), "evil\\`tag")
}

func TestManageOwners_NothingToChange(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{"a1": writeTestAsset()}}
	ft := &fakeTeamService{owners: []Owner{{Type: "team", ID: "t1"}}}
	tc := writeToolContext(fa, ft)

	res, _, _ := tc.manageOwners(context.Background(), nil, ManageOwnersInput{ID: "a1", Action: "add", OwnerType: "team", OwnerID: "t1", Confirm: true})
	assert.True(t, res.IsError)
	assert.Contains(t, resultText(t, res), "Nothing to change")
	assert.Empty(t, ft.added)

	res, _, _ = tc.manageOwners(context.Background(), nil, ManageOwnersInput{ID: "a1", Action: "remove", OwnerType: "user", OwnerID: "u9", Confirm: true})
	assert.True(t, res.IsError)
	assert.Empty(t, ft.removed)
}

func TestManageOwners_ProposeThenApply(t *testing.T) {
	fa := &fakeAssetService{assets: map[string]*asset.Asset{"a1": writeTestAsset()}}
	ft := &fakeTeamService{}
	tc := writeToolContext(fa, ft)

	res, _, _ := tc.manageOwners(context.Background(), nil, ManageOwnersInput{ID: "a1", Action: "add", OwnerType: "team", OwnerID: "t`1"})
	text := resultText(t, res)
	assert.Contains(t, text, "Proposed change")
	assert.Contains(t, text, "t\\`1", "the owner id must be markdown-escaped in the preview")
	assert.Empty(t, ft.added, "a preview must not write")

	res, _, _ = tc.manageOwners(context.Background(), nil, ManageOwnersInput{ID: "a1", Action: "add", OwnerType: "team", OwnerID: "t1", Confirm: true})
	assert.Contains(t, resultText(t, res), "Change applied")
	assert.Equal(t, []string{"team:t1"}, ft.added)
}
