package azureblob

import "testing"

func TestContainerMRN(t *testing.T) {
	s := &Source{config: &Config{}}
	if got, want := s.containerMRN("my-name"), "mrn://asset/container/azureblob/my-name"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	s.config.Zone = "prod-eu"
	if got, want := s.containerMRN("my-name"), "mrn://asset/container/azureblob@prod-eu/my-name"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
