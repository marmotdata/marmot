package settings

import (
	"context"
	"testing"

	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestSettingsStore_Postgres(t *testing.T) {
	ctx := context.Background()
	store := NewPostgresStore(pgtest.TempDB(t))

	got, err := GetSignIn(ctx, store)
	if err != nil || got != DefaultSignIn() {
		t.Fatalf("unsaved sign-in settings = %+v, %v; want the defaults", got, err)
	}

	want := SignIn{CreateOnFirstSSO: false, DefaultRole: "admin", DefaultTeamID: "", AllowPasswordSignIn: false}
	if err := store.Set(ctx, KeySignIn, want, ""); err != nil {
		t.Fatal(err)
	}
	if got, err := GetSignIn(ctx, store); err != nil || got != want {
		t.Fatalf("saved sign-in settings = %+v, %v", got, err)
	}

	if err := SetProviderEnabled(ctx, store, "okta", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetProviderEnabled(ctx, store, "google", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := SetProviderEnabled(ctx, store, "okta", true, ""); err != nil {
		t.Fatal(err)
	}
	off, err := Switches{Store: store}.DisabledProviders(ctx)
	if err != nil || len(off) != 1 || !off["google"] {
		t.Fatalf("disabled providers = %v, %v", off, err)
	}

	if last, err := LastFullReindex(ctx, store); err != nil || last != nil {
		t.Fatalf("last reindex before any = %v, %v", last, err)
	}
}
