package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code and bytes written.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Logger returns an HTTP middleware that logs requests with timing and status.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := newResponseWriter(w)
		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		statusColor := colorForStatus(wrapped.statusCode)
		methodColor := colorForMethod(r.Method)

		log.Printf("%s %s %s %s %s %s %d bytes",
			statusColor(fmt.Sprintf("%d", wrapped.statusCode)),
			methodColor(fmt.Sprintf("%-7s", r.Method)),
			r.URL.Path,
			fmt.Sprintf("%13v", duration),
			extractIP(r),
			r.UserAgent(),
			wrapped.bytesWritten,
		)
	})
}

func colorForStatus(code int) func(string) string {
	switch {
	case code >= 200 && code < 300:
		return green
	case code >= 300 && code < 400:
		return cyan
	case code >= 400 && code < 500:
		return yellow
	default:
		return red
	}
}

func colorForMethod(method string) func(string) string {
	switch method {
	case "GET":
		return blue
	case "POST":
		return green
	case "PUT":
		return yellow
	case "DELETE":
		return red
	case "PATCH":
		return cyan
	default:
		return white
	}
}

// ANSI color helpers
func green(s string) string  { return "\033[32m" + s + "\033[0m" }
func cyan(s string) string   { return "\033[36m" + s + "\033[0m" }
func yellow(s string) string { return "\033[33m" + s + "\033[0m" }
func red(s string) string    { return "\033[31m" + s + "\033[0m" }
func blue(s string) string   { return "\033[34m" + s + "\033[0m" }
func white(s string) string  { return "\033[37m" + s + "\033[0m" }
