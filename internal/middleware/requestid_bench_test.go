package middleware

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Current implementation: compiled regexp.
var benchRegex = regexp.MustCompile(`^[a-zA-Z0-9\-_]{1,64}$`)

func isValidRequestIDLoop(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// Test inputs covering realistic request-ID patterns.
var benchIDs = []struct {
	name string
	id   string
}{
	{"short", "abc123"},
	{"uuid", "550e8400-e29b-41d4-a716-446655440000"},
	{"max64", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	{"reject_too_long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, // 65 chars
	{"reject_invalid", "hello world!"},
}

// --- Regex ---

func BenchmarkValidateRequestID_Regex(b *testing.B) {
	for _, tc := range benchIDs {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = benchRegex.MatchString(tc.id)
			}
		})
	}
}

// --- Manual loop ---

func BenchmarkValidateRequestID_Loop(b *testing.B) {
	for _, tc := range benchIDs {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = isValidRequestIDLoop(tc.id)
			}
		})
	}
}

// --- Parallel variants (simulate real gateway concurrency) ---

func BenchmarkValidateRequestID_Regex_Parallel(b *testing.B) {
	// UUID is the most common real-world request ID format.
	id := "550e8400-e29b-41d4-a716-446655440000"
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = benchRegex.MatchString(id)
		}
	})
}

func BenchmarkValidateRequestID_Loop_Parallel(b *testing.B) {
	id := "550e8400-e29b-41d4-a716-446655440000"
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = isValidRequestIDLoop(id)
		}
	})
}

// Correctness sanity check — both implementations must agree.
func TestValidateRequestID_LoopMatchesRegex(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"", false},
		{"a", true},
		{"abc-123_XYZ", true},
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},   // 64 — valid
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}, // 65 — too long
		{"hello world", false},
		{"café", false},
		{"a/b", false},
		{"a\x00b", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, isValidRequestIDLoop(tc.id), "id=%q", tc.id)
		assert.Equal(t, tc.want, isValidRequestID(tc.id), "isValidRequestID(%q)", tc.id)
	}
}
