package platform

import (
	"encoding/json"
	"net/http"
)

// Envelope 是两个 API 共用的响应外壳，方便 Nginx 日志和调用方按 code 判断业务结果。
type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Code: code, Message: message, Data: data})
}
