package asset

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/marmotdata/marmot/internal/core/metamodel"
)

const governedProfile = `formatVersion: 1
id: example
version: 1
defaultLocale: en
fields:
  - id: retention
    type: integer
    core: true
    required: true
    storage: metadata.example.retention
    validation:
      minimum: 1
    presentation:
      labelKey: example.retention.label
  - id: note
    type: string
    core: true
    required: false
    nullable: true
    storage: metadata.example.note
    presentation:
      labelKey: example.note.label
`

func TestNativeProfileLeavesContractsUnchanged(t *testing.T) {
	svc := NewService(newMemoryRepo())
	schema := svc.Metamodel("asset")
	if schema.Enabled {
		t.Fatal("native schema should not be marked enabled")
	}
	created, err := svc.Create(context.Background(), validCreate("native"))
	if err != nil {
		t.Fatal(err)
	}
	if created.Version != 1 || created.Name == nil || *created.Name != "native" {
		t.Fatalf("unexpected native create: %+v", created)
	}
}

func TestCreateAllowsMissingRequiredGovernedField(t *testing.T) {
	// A required governed field is completeness, not validity: discovery must be able to
	// create an asset before anyone can fill in a value it has no way to know.
	svc := newGovernedService(t)
	created, err := svc.Create(context.Background(), validCreate("table"))
	if err != nil {
		t.Fatalf("a missing governed required field must not block Create: %v", err)
	}
	registry := mustLoadProfile(t)
	missing := registry.Missing(MetamodelValues(registry, created), "asset", !created.IsStub)
	if len(missing) != 1 || missing[0].Field != "retention" {
		t.Fatalf("expected retention reported missing: %v", missing)
	}
}

func TestCreateStubSkipsGovernedRequired(t *testing.T) {
	svc := newGovernedService(t)
	input := validCreate("stub")
	input.IsStub = true
	if _, err := svc.Create(context.Background(), input); err != nil {
		t.Fatal(err)
	}
}

func TestPatchFieldsDoesNotRequireFullDocument(t *testing.T) {
	svc := newGovernedService(t)
	created := mustCreateGoverned(t, svc, 30)
	updated, err := svc.PatchFields(context.Background(), created.ID, created.Version, map[string]any{"retention": 90.0})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("version not incremented: %d", updated.Version)
	}
	got, _ := metadataValue(updated.Metadata, "metadata.example.retention")
	if got != 90.0 {
		t.Fatalf("retention not patched: %v", got)
	}
	if _, ok := metadataValue(updated.Metadata, "metadata.plugin.extra"); !ok {
		t.Fatal("unknown metadata was dropped")
	}
}

func TestLegacyUpdatePreservesGovernedMetadata(t *testing.T) {
	svc := newGovernedService(t)
	created := mustCreateGoverned(t, svc, 30)
	tags := []string{"keep"}
	updated, err := svc.Update(context.Background(), created.ID, UpdateInput{
		Tags:     tags,
		Metadata: map[string]any{"plugin": map[string]any{"extra": "yes", "other": 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := metadataValue(updated.Metadata, "metadata.example.retention")
	if !ok || got != 30.0 {
		t.Fatalf("governed field was wiped: %v %v", got, ok)
	}
	if extra, ok := metadataValue(updated.Metadata, "metadata.plugin.extra"); !ok || extra != "yes" {
		t.Fatal("unrelated metadata was dropped")
	}
}

func TestLegacyUpdateCannotChangeGovernedFieldWithoutVersion(t *testing.T) {
	svc := newGovernedService(t)
	created := mustCreateGoverned(t, svc, 30)
	_, err := svc.Update(context.Background(), created.ID, UpdateInput{
		Metadata: map[string]any{"example": map[string]any{"retention": 1.0}},
	})
	if !errors.Is(err, ErrVersionRequired) {
		t.Fatalf("expected version required, got %v", err)
	}
}

func TestPatchFieldsDetectsVersionConflict(t *testing.T) {
	svc := newGovernedService(t)
	created := mustCreateGoverned(t, svc, 30)
	if _, err := svc.PatchFields(context.Background(), created.ID, created.Version, map[string]any{"retention": 40.0}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.PatchFields(context.Background(), created.ID, created.Version, map[string]any{"retention": 50.0})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestNullablePatchRemovesOptionalField(t *testing.T) {
	svc := newGovernedService(t)
	created := mustCreateGoverned(t, svc, 30)
	if _, err := svc.PatchFields(context.Background(), created.ID, created.Version, map[string]any{"note": "keep"}); err != nil {
		t.Fatal(err)
	}
	current, _ := svc.Get(context.Background(), created.ID)
	updated, err := svc.PatchFields(context.Background(), created.ID, current.Version, map[string]any{"note": nil})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := metadataValue(updated.Metadata, "metadata.example.note"); ok {
		t.Fatal("nullable field was not removed")
	}
}

func TestAddTagOnLegacyAssetMissingRequiredFieldStillSyncs(t *testing.T) {
	// An asset created before the profile added `retention` has no way to have a value for
	// it. Re-validating on every write must not stop it from syncing forever.
	repo := newMemoryRepo()
	svc := NewService(repo, WithMetamodel(mustLoadProfile(t)))
	name, mrn := "legacy", "mrn:asset:legacy"
	if err := repo.Create(context.Background(), &Asset{
		ID: "legacy-id", Version: 1, Name: &name, MRN: &mrn, Type: "table",
		Providers: []string{"test"}, CreatedBy: "tester", Metadata: map[string]any{},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddTag(context.Background(), "legacy-id", "x"); err != nil {
		t.Fatalf("a legacy asset missing a governed required field must keep syncing: %v", err)
	}
}

func newGovernedService(t *testing.T) Service {
	t.Helper()
	return NewService(newMemoryRepo(), WithMetamodel(mustLoadProfile(t)))
}

type patchObserver struct{ fields []string }

func (o *patchObserver) OnAssetUpdated(_ context.Context, _ *Asset, _ string, fields []string) {
	o.fields = fields
}
func (*patchObserver) OnAssetDeleted(context.Context, *Asset) {}

func TestPatchNotifiesAndRejectsUnimplementedRelations(t *testing.T) {
	svc := newGovernedService(t)
	created := mustCreateGoverned(t, svc, 30)
	observer := &patchObserver{}
	svc.SetNotificationObserver(observer)
	updated, err := svc.PatchFields(context.Background(), created.ID, created.Version, map[string]any{"retention": 90.0})
	if err != nil {
		t.Fatal(err)
	}
	if len(observer.fields) != 1 || observer.fields[0] != FieldMetadata {
		t.Fatalf("notification fields: %v", observer.fields)
	}
	for _, fields := range []map[string]any{{"owners": []any{"owner"}}, {"business_area": "area"}, {"retention": nil}, {"retention": "invalid"}} {
		if _, err := svc.PatchFields(context.Background(), created.ID, updated.Version, fields); err == nil {
			t.Fatalf("accepted unsupported/invalid fields: %v", fields)
		}
	}
	persisted, err := svc.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Version != updated.Version || *persisted.MRN != *created.MRN {
		t.Fatal("failed patch modified persisted identity or version")
	}
}

func mustLoadProfile(t *testing.T) *metamodel.Registry {
	t.Helper()
	r, err := metamodel.Load(strings.NewReader(governedProfile))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func validCreate(name string) CreateInput {
	mrn := "mrn:asset:" + name
	return CreateInput{
		Name:      &name,
		MRN:       &mrn,
		Type:      "table",
		Providers: []string{"test"},
		CreatedBy: "tester",
		Metadata:  map[string]any{"plugin": map[string]any{"extra": "yes"}},
	}
}

func mustCreateGoverned(t *testing.T, svc Service, retention float64) *Asset {
	t.Helper()
	input := validCreate("table")
	input.Metadata["example"] = map[string]any{"retention": retention}
	created, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return created
}

type memoryRepo struct {
	mu    sync.Mutex
	byID  map[string]*Asset
	byMRN map[string]*Asset
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{byID: map[string]*Asset{}, byMRN: map[string]*Asset{}}
}

func cloneAsset(a *Asset) *Asset {
	data, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	var out Asset
	if err := json.Unmarshal(data, &out); err != nil {
		panic(err)
	}
	return &out
}

func (m *memoryRepo) Create(_ context.Context, a *Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if a.MRN != nil {
		if _, exists := m.byMRN[strings.ToLower(*a.MRN)]; exists {
			return ErrConflict
		}
	}
	stored := cloneAsset(a)
	m.byID[a.ID] = stored
	if a.MRN != nil {
		m.byMRN[strings.ToLower(*a.MRN)] = stored
	}
	return nil
}

func (m *memoryRepo) Get(_ context.Context, id string) (*Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneAsset(stored), nil
}

func (m *memoryRepo) GetByMRN(_ context.Context, qualifiedName string) (*Asset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.byMRN[strings.ToLower(qualifiedName)]
	if !ok || stored.IsStub {
		return nil, ErrNotFound
	}
	return cloneAsset(stored), nil
}

func (m *memoryRepo) Update(_ context.Context, a *Asset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.byID[a.ID]
	if !ok {
		return ErrNotFound
	}
	if stored.Version != a.Version {
		return ErrVersionConflict
	}
	a.Version++
	next := cloneAsset(a)
	m.byID[a.ID] = next
	if next.MRN != nil {
		m.byMRN[strings.ToLower(*next.MRN)] = next
	}
	return nil
}

func (m *memoryRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.byID[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	if stored.MRN != nil {
		delete(m.byMRN, strings.ToLower(*stored.MRN))
	}
	return nil
}

func (m *memoryRepo) DeleteByMRN(ctx context.Context, mrn string) error {
	got, err := m.GetByMRN(ctx, mrn)
	if err != nil {
		return err
	}
	return m.Delete(ctx, got.ID)
}

func (m *memoryRepo) Search(context.Context, SearchFilter, bool) ([]*Asset, int, AvailableFilters, error) {
	return nil, 0, AvailableFilters{}, nil
}
func (m *memoryRepo) GetMyAssets(context.Context, string, []string, int, int) ([]*Asset, int, error) {
	return nil, 0, nil
}
func (m *memoryRepo) Summary(context.Context) (*AssetSummary, error) { return &AssetSummary{}, nil }
func (m *memoryRepo) ListByPattern(context.Context, string, string) ([]*Asset, error) {
	return nil, nil
}
func (m *memoryRepo) GetByMRNs(context.Context, []string) ([]*Asset, error) { return nil, nil }
func (m *memoryRepo) GetByTypeAndName(context.Context, string, string) (*Asset, error) {
	return nil, ErrNotFound
}
func (m *memoryRepo) GetMetadataFieldsWithContext(context.Context, *MetadataContext) ([]MetadataFieldSuggestion, error) {
	return nil, nil
}
func (m *memoryRepo) GetMetadataValuesWithContext(context.Context, string, string, int, *MetadataContext) ([]MetadataValueSuggestion, error) {
	return nil, nil
}
func (m *memoryRepo) GetMetadataFields(context.Context) ([]MetadataFieldSuggestion, error) {
	return nil, nil
}
func (m *memoryRepo) GetMetadataValues(context.Context, string, string, int) ([]MetadataValueSuggestion, error) {
	return nil, nil
}
func (m *memoryRepo) GetTagSuggestions(context.Context, string, int) ([]string, error) {
	return nil, nil
}
func (m *memoryRepo) GetRunHistory(context.Context, string, int, int) ([]*RunHistory, int, error) {
	return nil, 0, nil
}
func (m *memoryRepo) GetRunHistoryHistogram(context.Context, string, int) ([]HistogramBucket, error) {
	return nil, nil
}
func (m *memoryRepo) AddTerms(context.Context, string, []string, string, string) error { return nil }
func (m *memoryRepo) RemoveTerm(context.Context, string, string) error                 { return nil }
func (m *memoryRepo) GetTerms(context.Context, string) ([]AssetTerm, error)            { return nil, nil }
func (m *memoryRepo) GetAssetsByTerm(context.Context, string, int, int) ([]*Asset, int, error) {
	return nil, 0, nil
}

var _ Repository = (*memoryRepo)(nil)
