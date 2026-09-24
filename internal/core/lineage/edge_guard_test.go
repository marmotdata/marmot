package lineage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/marmotdata/marmot/internal/core/asset"
)

type edgeRepo struct {
	Repository
	edges [][2]string
}

func (r *edgeRepo) EdgeExists(context.Context, string, string) (bool, error) { return false, nil }

func (r *edgeRepo) CreateDirectLineage(_ context.Context, source, target, _, _ string) (string, error) {
	r.edges = append(r.edges, [2]string{source, target})
	return "edge", nil
}

func (r *edgeRepo) StoreRunHistory(context.Context, *RunHistoryEntry) error { return nil }

type stubAssets struct {
	asset.Service
	byMRN map[string]*asset.Asset
}

func (a *stubAssets) GetByMRN(_ context.Context, mrn string) (*asset.Asset, error) {
	if found, ok := a.byMRN[mrn]; ok {
		return found, nil
	}
	return nil, asset.ErrAssetNotFound
}

func (a *stubAssets) Create(_ context.Context, in asset.CreateInput) (*asset.Asset, error) {
	created := &asset.Asset{ID: *in.MRN, MRN: in.MRN}
	a.byMRN[*in.MRN] = created
	return created, nil
}

var errRefused = errors.New("refused")

// refuseTargets refuses every edge whose target contains marker.
func refuseTargets(marker string) EdgeGuard {
	return func(_ context.Context, _, target string) error {
		if strings.Contains(target, marker) {
			return errRefused
		}
		return nil
	}
}

func TestEdgeGuardVetsDirectLineage(t *testing.T) {
	repo := &edgeRepo{}
	svc := NewService(repo, &stubAssets{byMRN: map[string]*asset.Asset{}}, WithEdgeGuard(refuseTargets("secret")))
	if _, err := svc.CreateDirectLineage(context.Background(), "mrn://a", "mrn://secret", "DIRECT", ""); !errors.Is(err, errRefused) {
		t.Fatalf("err = %v, want the guard's refusal", err)
	}
	if _, err := svc.CreateDirectLineage(context.Background(), "mrn://a", "mrn://b", "DIRECT", ""); err != nil {
		t.Fatal(err)
	}
	if len(repo.edges) != 1 {
		t.Fatalf("edges = %v", repo.edges)
	}
}

func TestOpenLineageSkipsRefusedEdges(t *testing.T) {
	repo := &edgeRepo{}
	svc := NewService(repo, &stubAssets{byMRN: map[string]*asset.Asset{}}, WithEdgeGuard(refuseTargets("forbidden")))
	event := &RunEvent{
		EventType: "COMPLETE",
		Job:       Job{Namespace: "etl", Name: "daily"},
		Inputs:    []Dataset{{Namespace: "postgres://db", Name: "public.orders"}},
		Outputs:   []Dataset{{Namespace: "postgres://db", Name: "public.report"}, {Namespace: "postgres://db", Name: "public.forbidden"}},
	}
	if err := svc.ProcessOpenLineageEvent(context.Background(), event, "emitter"); err != nil {
		t.Fatalf("a refused edge must not fail the event: %v", err)
	}
	var targets []string
	for _, e := range repo.edges {
		targets = append(targets, e[1])
	}
	if len(repo.edges) != 2 || strings.Contains(strings.Join(targets, " "), "forbidden") {
		t.Fatalf("edges = %v, want the input and the allowed output only", repo.edges)
	}
}
