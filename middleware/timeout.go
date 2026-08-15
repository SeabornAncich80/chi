package middleware

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

// Timeout is a middleware that cancels ctx after duration.
func Timeout(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			r = r.WithContext(ctx)

			done := make(chan struct{})
			panicChan := make(chan interface{}, 1)

			ww := NewWrapResponseWriter(w, r.ProtoMajor)
			tw := &timeoutWriter{WrapResponseWriter: ww}

			go func() {
				defer func() {
					if p := recover(); p != nil {
						panicChan <- p
					}
				}()
				next.ServeHTTP(tw, r)
				close(done)
			}()

			select {
			case <-done:
				// completed normally
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					tw.mu.Lock()
					if !tw.wroteHeader {
						tw.timedOut = true
						tw.WrapResponseWriter.WriteHeader(http.StatusGatewayTimeout)
						tw.WrapResponseWriter.Write([]byte(http.StatusText(http.StatusGatewayTimeout)))
					}
					tw.mu.Unlock()
				}
			case p := <-panicChan:
				panic(p)
			}
		}
		return http.HandlerFunc(fn)
	}
}

type timeoutWriter struct {
	WrapResponseWriter
	mu          sync.Mutex
	timedOut    bool
	wroteHeader bool
}

func (tw *timeoutWriter) WriteHeader(code int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut || tw.wroteHeader {
		return
	}
	tw.wroteHeader = true
	tw.WrapResponseWriter.WriteHeader(code)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return 0, http.ErrHandlerTimeout
	}
	tw.wroteHeader = true
	return tw.WrapResponseWriter.Write(b)
}

func (tw *timeoutWriter) Flush() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.timedOut {
		return
	}
	tw.WrapResponseWriter.Flush()
}

func (tw *timeoutWriter) Status() int {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.WrapResponseWriter.Status()
}

func (tw *timeoutWriter) BytesWritten() int {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	return tw.WrapResponseWriter.BytesWritten()
}

func (tw *timeoutWriter) Tee(writer io.Writer) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	tw.WrapResponseWriter.Tee(writer)
}
