package lambda

import "testing"

func TestFunctionMRN(t *testing.T) {
	s := &Source{config: &Config{}}
	if got, want := s.functionMRN("my-name"), "mrn://asset/function/lambda/my-name"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	s.config.Zone = "prod-eu"
	if got, want := s.functionMRN("my-name"), "mrn://asset/function/lambda@prod-eu/my-name"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
