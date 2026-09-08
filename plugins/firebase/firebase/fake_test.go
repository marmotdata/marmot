package firebase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	firebaseapi "google.golang.org/api/firebase/v1beta1"
	firebasedatabase "google.golang.org/api/firebasedatabase/v1beta"
	firestoreadmin "google.golang.org/api/firestore/v1"
)

// The Firebase admin plane has no emulator: the Firestore emulator answers
// 404 for the database listing and there is no Realtime Database or project
// emulator at all. These tests serve those three calls from a fake built out
// of the same generated structs the real clients decode into, so the response
// shapes cannot drift from the API.

// fakeAdmin is a stand-in for the three Firebase admin APIs. A nil field
// means "this call fails", which is how the tests exercise a project the
// caller only has partial access to.
type fakeAdmin struct {
	databases *firestoreadmin.GoogleFirestoreAdminV1ListDatabasesResponse
	instances *firebasedatabase.ListDatabaseInstancesResponse
	project   *firebaseapi.FirebaseProject

	mu             sync.Mutex
	requestedPaths []string
}

// paths returns the request paths seen so far. The server answers each
// request on its own goroutine, so the list needs a lock.
func (f *fakeAdmin) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requestedPaths...)
}

func (f *fakeAdmin) start(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requestedPaths = append(f.requestedPaths, r.URL.Path)
		f.mu.Unlock()

		switch {
		case strings.HasSuffix(r.URL.Path, "/databases"):
			writeOrFail(w, f.databases)
		case strings.HasSuffix(r.URL.Path, "/instances"):
			writeOrFail(w, f.instances)
		case strings.HasPrefix(r.URL.Path, "/v1beta1/projects/"):
			writeOrFail(w, f.project)
		default:
			http.Error(w, `{"error":{"code":404,"message":"not found"}}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	return server.URL
}

func writeOrFail(w http.ResponseWriter, body any) {
	if body == nil || isNilPointer(body) {
		http.Error(w, `{"error":{"code":403,"message":"permission denied"}}`, http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// isNilPointer spots a typed nil handed in through an any, which a plain
// `body == nil` comparison misses.
func isNilPointer(body any) bool {
	switch value := body.(type) {
	case *firestoreadmin.GoogleFirestoreAdminV1ListDatabasesResponse:
		return value == nil
	case *firebasedatabase.ListDatabaseInstancesResponse:
		return value == nil
	case *firebaseapi.FirebaseProject:
		return value == nil
	}
	return false
}

// Response bodies below are trimmed copies of the shapes the real APIs
// return, filled into the generated structs.

func fakeDatabases() *firestoreadmin.GoogleFirestoreAdminV1ListDatabasesResponse {
	return &firestoreadmin.GoogleFirestoreAdminV1ListDatabasesResponse{
		Databases: []*firestoreadmin.GoogleFirestoreAdminV1Database{
			{
				Name:                          "projects/marmot-demo/databases/(default)",
				LocationId:                    "nam5",
				Type:                          "FIRESTORE_NATIVE",
				ConcurrencyMode:               "PESSIMISTIC",
				PointInTimeRecoveryEnablement: "POINT_IN_TIME_RECOVERY_DISABLED",
				DeleteProtectionState:         "DELETE_PROTECTION_DISABLED",
				VersionRetentionPeriod:        "3600s",
				EarliestVersionTime:           "2026-09-08T10:00:00Z",
				CreateTime:                    "2024-01-02T03:04:05Z",
				UpdateTime:                    "2026-05-06T07:08:09Z",
				Uid:                           "7c6f1f4e-2c1f-4a52-9a1e-3b7c0a5d9e11",
				FreeTier:                      true,
			},
			{
				Name:                  "projects/marmot-demo/databases/analytics",
				LocationId:            "europe-west1",
				Type:                  "DATASTORE_MODE",
				DeleteProtectionState: "DELETE_PROTECTION_ENABLED",
				Uid:                   "a1b2c3d4-0000-4a52-9a1e-3b7c0a5d9e22",
			},
		},
	}
}

func fakeInstances() *firebasedatabase.ListDatabaseInstancesResponse {
	return &firebasedatabase.ListDatabaseInstancesResponse{
		Instances: []*firebasedatabase.DatabaseInstance{
			{
				Name:        "projects/451234567890/locations/us-central1/instances/marmot-demo-default-rtdb",
				DatabaseUrl: "https://marmot-demo-default-rtdb.firebaseio.com",
				Project:     "projects/451234567890",
				State:       "ACTIVE",
				Type:        "DEFAULT_DATABASE",
			},
			{
				Name:        "projects/451234567890/locations/europe-west1/instances/marmot-demo-events",
				DatabaseUrl: "https://marmot-demo-events.europe-west1.firebasedatabase.app",
				Project:     "projects/451234567890",
				State:       "DISABLED",
				Type:        "USER_DATABASE",
			},
		},
	}
}

func fakeProject() *firebaseapi.FirebaseProject {
	return &firebaseapi.FirebaseProject{
		Name:          "projects/marmot-demo",
		ProjectId:     "marmot-demo",
		ProjectNumber: 451234567890,
		DisplayName:   "Marmot Demo",
		State:         "ACTIVE",
	}
}
