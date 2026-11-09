package gcs

import "testing"

func TestBucketMRN(t *testing.T) {
	s := &Source{config: &Config{}}
	if got, want := s.bucketMRN("my-name"), "mrn://asset/bucket/gcs/my-name"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	s.config.Zone = "prod-eu"
	if got, want := s.bucketMRN("my-name"), "mrn://asset/bucket/gcs@prod-eu/my-name"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
