package middleware

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// prebuilt holds pre-serialised JSON for known (code, message) pairs
// so that the hot error paths allocate nothing.
var prebuilt = func() map[int]map[string][]byte {
	pairs := []struct {
		code int
		msg  string
	}{
		{http.StatusUnauthorized, "missing authorization header"},
		{http.StatusUnauthorized, "invalid authorization format"},
		{http.StatusUnauthorized, "invalid or expired token"},
		{http.StatusForbidden, "no token in context"}, // routecheck: token missing from context
		{http.StatusForbidden, "route not allowed"},
		{http.StatusTooManyRequests, "rate limit exceeded"},
		{http.StatusRequestEntityTooLarge, "request body too large"},
		{http.StatusInternalServerError, "internal server error"},
		{http.StatusInternalServerError, "no token in context"}, // ratelimit: token missing (should not happen post-auth)
	}

	m := make(map[int]map[string][]byte, len(pairs))
	for _, p := range pairs {
		b, err := json.Marshal(errorResponse{Error: errorBody{Code: p.code, Message: p.msg}})
		if err != nil {
			panic("prebuilt response marshal: " + err.Error())
		}
		b = append(b, '\n')
		if m[p.code] == nil {
			m[p.code] = make(map[string][]byte)
		}
		m[p.code][p.msg] = b
	}
	return m
}()

func writeJSON(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	if msgs, ok := prebuilt[code]; ok {
		if b, ok := msgs[message]; ok {
			_, _ = w.Write(b)
			return
		}
	}

	// Fallback for unexpected combinations.
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{Code: code, Message: message},
	})
}
