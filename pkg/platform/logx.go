package platform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Logger 把日志打成单行 JSON，方便 Promtail 直接送进 Loki。
type Logger struct {
	service string
	out     io.Writer
}

func NewLogger(service string) Logger {
	return Logger{service: service, out: os.Stdout}
}

func (l Logger) Info(msg string, kv ...any)  { l.write("info", msg, kv) }
func (l Logger) Error(msg string, kv ...any) { l.write("error", msg, kv) }

func (l Logger) write(level, msg string, kv []any) {
	rec := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"level":   level,
		"service": l.service,
		"msg":     msg,
	}
	for i := 0; i+1 < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			continue
		}
		val := kv[i+1]
		if err, ok := val.(error); ok {
			val = err.Error()
		}
		rec[key] = val
	}
	encoded, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(l.out, "{\"level\":\"error\",\"service\":%q,\"msg\":\"log marshal failed\"}\n", l.service)
		return
	}
	fmt.Fprintln(l.out, string(encoded))
}
