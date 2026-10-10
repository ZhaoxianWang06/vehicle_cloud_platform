package platform

import (
	"fmt"
	"net/http"
	"time"
)

// StatusRecorder 记下最终 HTTP 状态码，供访问日志和指标使用。
type StatusRecorder struct {
	http.ResponseWriter
	Status int
}

func (s *StatusRecorder) WriteHeader(code int) {
	s.Status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *StatusRecorder) Write(b []byte) (int, error) {
	if s.Status == 0 {
		s.Status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// AccessLog 输出一行访问日志。不记录请求体，避免车控参数进日志。
func AccessLog(log Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &StatusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.Status == 0 {
			rec.Status = http.StatusOK
		}
		log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.Status,
			"latency_ms", time.Since(start).Milliseconds(),
			"rid", r.Header.Get("X-Request-Id"),
		)
	})
}

// Recover 保证单个请求 panic 不会打垮进程。车控副本少，进程活着比一次请求成功更重要。
func Recover(log Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic", "err", fmt.Sprint(recovered), "path", r.URL.Path)
				WriteJSON(w, http.StatusInternalServerError, 50000, "内部错误", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
