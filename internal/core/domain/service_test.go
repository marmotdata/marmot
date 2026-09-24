package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/marmotdata/marmot/internal/core/domain"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func newService(t *testing.T) (domain.Service, func() string) {
	t.Helper()
	pool := pgtest.TempDB(t)
	return domain.NewService(domain.NewPostgresRepository(pool)), func() string { return pgtest.SeedAsset(t, pool) }
}

func mustCreate(t *testing.T, svc domain.Service, name string, parent *domain.Domain) *domain.Domain {
	t.Helper()
	in := domain.CreateInput{Name: name}
	if parent != nil {
		in.ParentID = &parent.ID
	}
	d, err := svc.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	return d
}

func wantErr(t *testing.T, err, target error) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("err = %v, want %v", err, target)
	}
}

func TestCreateBuildsPaths(t *testing.T) {
	svc, _ := newService(t)
	finance := mustCreate(t, svc, "  Finance  ", nil)
	payments := mustCreate(t, svc, "Payments", finance)

	if finance.Name != "Finance" || finance.Depth != 1 || finance.Path != "/"+finance.ID+"/" {
		t.Fatalf("root = %+v", finance)
	}
	if payments.Depth != 2 || payments.Path != finance.Path+payments.ID+"/" || *payments.ParentID != finance.ID {
		t.Fatalf("child = %+v", payments)
	}
	_, err := svc.Create(context.Background(), domain.CreateInput{Name: "payments", ParentID: &finance.ID})
	wantErr(t, err, domain.ErrNameConflict)
	_, err = svc.Create(context.Background(), domain.CreateInput{Name: " "})
	wantErr(t, err, domain.ErrInvalidInput)
}

func TestMoveRewritesTheSubtree(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	a := mustCreate(t, svc, "A", nil)
	b := mustCreate(t, svc, "B", a)
	c := mustCreate(t, svc, "C", b)
	d := mustCreate(t, svc, "D", nil)

	moved, err := svc.Move(ctx, b.ID, &d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Path != d.Path+b.ID+"/" || moved.Depth != 2 || *moved.ParentID != d.ID {
		t.Fatalf("moved = %+v", moved)
	}
	child, err := svc.Get(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if child.Path != moved.Path+c.ID+"/" || child.Depth != 3 || *child.ParentID != b.ID {
		t.Fatalf("descendant = %+v", child)
	}

	root, err := svc.Move(ctx, b.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if root.ParentID != nil || root.Depth != 1 || root.Path != "/"+b.ID+"/" {
		t.Fatalf("moved to root = %+v", root)
	}

	_, err = svc.Move(ctx, b.ID, &c.ID)
	wantErr(t, err, domain.ErrCycle)
	_, err = svc.Move(ctx, b.ID, &b.ID)
	wantErr(t, err, domain.ErrCycle)

	mustCreate(t, svc, "B", d)
	_, err = svc.Move(ctx, b.ID, &d.ID)
	wantErr(t, err, domain.ErrNameConflict)
}

func TestDepthIsBounded(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	chain := []*domain.Domain{mustCreate(t, svc, "L1", nil)}
	for len(chain) < domain.MaxDepth {
		chain = append(chain, mustCreate(t, svc, "L", chain[len(chain)-1]))
	}
	_, err := svc.Create(ctx, domain.CreateInput{Name: "too deep", ParentID: &chain[len(chain)-1].ID})
	wantErr(t, err, domain.ErrTooDeep)

	other := mustCreate(t, svc, "Other", nil)
	mustCreate(t, svc, "Child", other)
	_, err = svc.Move(ctx, other.ID, &chain[len(chain)-2].ID)
	wantErr(t, err, domain.ErrTooDeep)
}

func TestDeleteRequiresAnEmptyLeaf(t *testing.T) {
	svc, seedAsset := newService(t)
	ctx := context.Background()
	parent := mustCreate(t, svc, "Parent", nil)
	leaf := mustCreate(t, svc, "Leaf", parent)

	wantErr(t, svc.Delete(ctx, parent.ID), domain.ErrHasChildren)

	asset := seedAsset()
	if err := svc.Assign(ctx, domain.KindAsset, asset, leaf.ID); err != nil {
		t.Fatal(err)
	}
	wantErr(t, svc.Delete(ctx, leaf.ID), domain.ErrNotEmpty)

	if err := svc.Assign(ctx, domain.KindAsset, asset, domain.UnassignedID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, leaf.ID); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Get(ctx, leaf.ID)
	wantErr(t, err, domain.ErrNotFound)
}

func TestMembership(t *testing.T) {
	svc, seedAsset := newService(t)
	ctx := context.Background()
	finance := mustCreate(t, svc, "Finance", nil)
	asset := seedAsset()

	got, err := svc.DomainOf(ctx, domain.KindAsset, asset)
	if err != nil || got != domain.UnassignedID {
		t.Fatalf("without a row: %q, %v; want unassigned", got, err)
	}
	if err := svc.Assign(ctx, domain.KindAsset, asset, finance.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.DomainOf(ctx, domain.KindAsset, asset); got != finance.ID {
		t.Fatalf("after assign: %q", got)
	}
	if err := svc.Assign(ctx, domain.KindAsset, asset, domain.UnassignedID); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.DomainOf(ctx, domain.KindAsset, asset); got != domain.UnassignedID {
		t.Fatalf("after unassign: %q", got)
	}

	wantErr(t, svc.Assign(ctx, domain.KindAsset, "missing", finance.ID), domain.ErrEntityNotFound)
	wantErr(t, svc.Assign(ctx, domain.KindGlossaryTerm, "not-a-uuid", finance.ID), domain.ErrEntityNotFound)
	wantErr(t, svc.Assign(ctx, domain.KindAsset, asset, "00000000-0000-4000-8000-00000000ffff"), domain.ErrNotFound)
	wantErr(t, svc.Assign(ctx, "folder", asset, finance.ID), domain.ErrInvalidInput)
}

func TestUnassignedIsProtected(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	other := mustCreate(t, svc, "Other", nil)
	unassigned := domain.UnassignedID
	renamed := "Inbox"

	wantErr(t, svc.Delete(ctx, unassigned), domain.ErrProtected)
	_, err := svc.Move(ctx, unassigned, &other.ID)
	wantErr(t, err, domain.ErrProtected)
	_, err = svc.Update(ctx, unassigned, domain.UpdateInput{Name: &renamed})
	wantErr(t, err, domain.ErrProtected)
	_, err = svc.Create(ctx, domain.CreateInput{Name: "Child", ParentID: &unassigned})
	wantErr(t, err, domain.ErrProtected)
	_, err = svc.Move(ctx, other.ID, &unassigned)
	wantErr(t, err, domain.ErrProtected)

	desc := "Entities waiting for classification"
	if _, err := svc.Update(ctx, unassigned, domain.UpdateInput{Description: &desc}); err != nil {
		t.Fatalf("describing unassigned: %v", err)
	}
}

func TestGetWithInvalidID(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Get(context.Background(), "not-a-uuid")
	wantErr(t, err, domain.ErrNotFound)
}
