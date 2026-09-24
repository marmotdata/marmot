package glossary

import (
	"context"
	"errors"
	"testing"
)

var anOwner = []OwnerInput{{ID: "u1", Type: "user"}}

func create(name, parent string) ImportTerm {
	return ImportTerm{Create: CreateTermInput{Name: name, Definition: name + " definition", Owners: anOwner}, ParentName: parent}
}

func TestImportWritesParentsFirst(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()
	existing, err := svc.Create(ctx, CreateTermInput{Name: "Finance", Definition: "Money", Owners: anOwner})
	if err != nil {
		t.Fatal(err)
	}
	written, err := svc.Import(ctx, []ImportTerm{
		create("Card payment", "Payment"),
		create("Payment", "Finance"),
		{ExistingID: existing.ID, Update: UpdateTermInput{Definition: ptr("Money matters")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 3 {
		t.Fatalf("written = %d terms", len(written))
	}
	payment, card := repo.byName(t, "Payment"), repo.byName(t, "Card payment")
	if payment.ParentTermID == nil || *payment.ParentTermID != existing.ID || card.ParentTermID == nil || *card.ParentTermID != payment.ID {
		t.Fatalf("hierarchy not resolved: payment=%v card=%v", payment.ParentTermID, card.ParentTermID)
	}
	if repo.byName(t, "Finance").Definition != "Money matters" {
		t.Fatal("the update row was not applied")
	}
}

func TestImportRejectsCyclesAndUnknownParents(t *testing.T) {
	svc := NewService(newFakeRepo())
	ctx := context.Background()
	_, err := svc.Import(ctx, []ImportTerm{create("A", "B"), create("B", "A")})
	if !errors.Is(err, ErrImportCycle) {
		t.Fatalf("err = %v, want ErrImportCycle", err)
	}
	_, err = svc.Import(ctx, []ImportTerm{create("A", "Nowhere")})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func ptr(s string) *string { return &s }
