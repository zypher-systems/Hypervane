package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// The UI builds paths with encodeURIComponent, so IDs arrive percent-encoded.
// Handlers must see each ID decoded exactly once before the Proxmox client
// escapes it again.
func TestURLParamDecodesEncodedIDsOnce(t *testing.T) {
	cases := []struct {
		name, path, want string
	}{
		{"upid", "/tasks/UPID%3Azs-hv1%3A00159E45%3A00AB659E%3A6AB4BC6D%3Avzdump%3A%3Aroot%40pam%3A", "UPID:zs-hv1:00159E45:00AB659E:6AB4BC6D:vzdump::root@pam:"},
		{"ha sid", "/tasks/vm%3A100", "vm:100"},
		{"user", "/tasks/root%40pam", "root@pam"},
		{"cidr", "/tasks/10.0.0.0%2F24", "10.0.0.0/24"},
		{"literal percent stays encoded once", "/tasks/a%2525b", "a%25b"},
		{"plain", "/tasks/zs-hv1", "zs-hv1"},
		{"raw colons", "/tasks/vm:100", "vm:100"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			r := chi.NewRouter()
			r.Get("/tasks/{upid}", func(w http.ResponseWriter, r *http.Request) {
				got = urlParam(r, "upid")
			})
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
