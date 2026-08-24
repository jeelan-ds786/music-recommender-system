package httplog

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/logger"
	"github.com/jeelan-ds786/music-recommender-system/music-identity-gatekeeper/internal/reqid"
)

// maxLoggedBodyBytes bounds how much of a request/response body gets
// logged, so a large payload can't bloat the log file.
const maxLoggedBodyBytes = 4096

// sensitiveFields are JSON body keys whose values are never written to
// logs, even at debug level — raw passwords and tokens must never end up
// in a log file.
var sensitiveFields = map[string]bool{
	"password":      true,
	"refresh_token": true,
	"access_token":  true,
}

// Middleware logs every request as it comes in (method, path, headers,
// body) and every response as it goes out (status, headers, body). It must
// run after reqid.Middleware so the request already has an id in context.
func Middleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid, _ := reqid.FromContext(r.Context())

			logIncoming(log, rid, r)

			rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK, body: &bytes.Buffer{}}
			next.ServeHTTP(rec, r)

			logOutgoing(log, rid, r, rec)
		})
	}
}

// logIncoming and logOutgoing are named (not inline closures) so the
// logger's automatic file/method tagging reports something readable
// ("logIncoming"/"logOutgoing") instead of a synthetic closure name.
func logIncoming(log *logger.Logger, rid string, r *http.Request) {
	body := readAndRestoreBody(r)

	log.Error(
		rid,
		"incoming request method=%s path=%s headers=%s body=%s",
		r.Method,
		r.URL.Path,
		formatHeaders(r.Header),
		formatBody(body),
	)
}

func logOutgoing(log *logger.Logger, rid string, r *http.Request, rec *responseRecorder) {
	log.Error(
		rid,
		"outgoing response method=%s path=%s status=%d headers=%s body=%s",
		r.Method,
		r.URL.Path,
		rec.status,
		formatHeaders(rec.Header()),
		formatBody(rec.body.Bytes()),
	)
}

// responseRecorder wraps http.ResponseWriter to capture the status code and
// body while still passing everything through to the real client.
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func readAndRestoreBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	return body
}

func formatHeaders(h http.Header) string {
	clone := h.Clone()
	if clone.Get("Authorization") != "" {
		clone.Set("Authorization", "REDACTED")
	}

	b, err := json.Marshal(clone)
	if err != nil {
		return "<unavailable>"
	}

	return string(b)
}

// formatBody redacts known-sensitive fields (password, refresh_token,
// access_token) before logging, and truncates long bodies. Non-JSON bodies
// are logged as-is (truncated), since there's no field structure to redact.
func formatBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err == nil {
		for k := range m {
			if sensitiveFields[strings.ToLower(k)] {
				m[k] = "***REDACTED***"
			}
		}

		if redacted, err := json.Marshal(m); err == nil {
			body = redacted
		}
	}

	if len(body) > maxLoggedBodyBytes {
		return string(body[:maxLoggedBodyBytes]) + "...(truncated)"
	}

	return string(body)
}
