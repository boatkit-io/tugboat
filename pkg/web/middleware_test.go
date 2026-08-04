package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestRequestMiddlewareLogsCompletedRequestAtDebug(t *testing.T) {
	t.Parallel()

	log := logrus.New()
	var output bytes.Buffer
	log.SetOutput(&output)
	handler := RequestMW(log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/status", nil))
	require.Empty(t, output.String())

	log.SetLevel(logrus.DebugLevel)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/status", nil))
	require.Contains(t, output.String(), "level=debug")
	require.Contains(t, output.String(), "completed request")
}

type streamingResponseRecorder struct {
	*httptest.ResponseRecorder
	writeDeadline time.Time
}

func (r *streamingResponseRecorder) SetWriteDeadline(deadline time.Time) error {
	r.writeDeadline = deadline
	return nil
}

func TestRequestMiddlewarePreservesStreamingSupport(t *testing.T) {
	t.Parallel()

	handler := RequestMW(logrus.New(), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)

		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		controller := http.NewResponseController(w)
		require.NoError(t, controller.SetWriteDeadline(time.Time{}))
	}))

	recorder := &streamingResponseRecorder{ResponseRecorder: httptest.NewRecorder()}
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/stream", nil))

	require.True(t, recorder.Flushed)
	require.True(t, recorder.writeDeadline.IsZero())
}
