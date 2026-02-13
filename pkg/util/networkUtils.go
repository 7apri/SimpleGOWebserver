package util

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"unicode"

	"github.com/bytedance/sonic"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func GetClientIP(r *http.Request) string {
	if ip := r.Header.Get("Cf-Connecting-Ip"); ip != "" {
		return ip
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func SendJson(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	err := sonic.ConfigDefault.NewEncoder(w).Encode(payload)
	if err != nil {
		log.Printf("JSON Encode Error: %v", err)
	}
}
func SendRawJsonSlice(w http.ResponseWriter, code int, payload []json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	w.Write([]byte{'['})
	for i, p := range payload {
		if i > 0 {
			w.Write([]byte{','})
		}
		w.Write(p)
	}
	w.Write([]byte{']'})
}
func SendErrorJson(w http.ResponseWriter, message string, code int) {
	type apiError struct {
		Error   string `json:"error"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	sonic.ConfigDefault.NewEncoder(w).Encode(
		apiError{
			Error:   http.StatusText(code),
			Code:    code,
			Message: message,
		})
}

func ParseGenericQuery[T any](mapper func([]string) T, queries ...string) []T {
	if len(queries) == 0 {
		return nil
	}

	minLen := -1
	for _, q := range queries {
		count := 1 + strings.Count(q, ",")
		if minLen == -1 || count < minLen {
			minLen = count
		}
	}

	results := make([]T, 0, minLen)
	rowBuffer := make([]string, len(queries))

	starts := make([]int, len(queries))

	for i := 0; i < minLen; i++ {
		for j, q := range queries {
			remainder := q[starts[j]:]
			commaIdx := strings.IndexByte(remainder, ',')

			var part string
			if commaIdx == -1 {
				part = remainder
				starts[j] = len(q)
			} else {
				part = remainder[:commaIdx]
				starts[j] += commaIdx + 1
			}

			rowBuffer[j] = strings.TrimSpace(part)
		}
		results = append(results, mapper(rowBuffer))
	}

	return results
}

func CleanQuery(input string) string {
	words := strings.Fields(input)
	s := strings.Join(words, " ")

	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)

	result, _, _ := transform.String(t, s)

	return strings.ToLower(RemoveWhiteSpaceUrl(result))
}

func RemoveWhiteSpaceUrl(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == ' ' {
			return '-'
		}
		return r
	}, s)
	return s
}
