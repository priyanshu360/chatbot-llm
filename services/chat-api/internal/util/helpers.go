package util

import "encoding/json"

func TruncateTitle(s string) string {
	runes := []rune(s)
	if len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return s
}

func ToJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
