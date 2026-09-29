package common

import (
	"github.com/marmotdata/marmot/pkg/config"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimitGroupsClientPorts(t *testing.T) {
	cfg := &config.Config{}
	cfg.RateLimit.Enabled = true
	handler := WithRateLimit(cfg, 1, 60)(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	path := "/test-totp-port-limit/" + strconv.FormatInt(time.Now().UnixNano(), 10)
	for i, addr := range []string{"192.0.2.117:41001", "192.0.2.117:41002"} {
		r := httptest.NewRequest("POST", path, nil)
		r.RemoteAddr = addr
		w := httptest.NewRecorder()
		handler(w, r)
		want := 204
		if i == 1 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("port rotation: got %d want %d", w.Code, want)
		}
	}
}
