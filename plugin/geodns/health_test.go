package geodns

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNoRedirect(t *testing.T) {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/redirected", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/redirected", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		srv := http.Server{
			Addr:    "localhost:8888",
			Handler: mux,
		}
		err := srv.ListenAndServe()
		require.NoError(t, err)
	}()

	checker := newHealthChecker()
	checker.port = "8888"

	condition := func() bool {
		return checker.checkOne("127.0.0.1")
	}

	require.Eventually(t, condition, 5*time.Second, 500*time.Millisecond)
}
