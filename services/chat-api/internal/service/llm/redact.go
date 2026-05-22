package llm

import "regexp"

var redactionPatterns = []struct {
	pattern *regexp.Regexp
	replace string
}{
	{regexp.MustCompile(`\b[\w.+-]+@[\w-]+\.\w{2,}\b`), "[EMAIL]"},
	{regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`), "[PHONE]"},
	{regexp.MustCompile(`\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`), "[CARD]"},
	{regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), "[IP]"},
	{regexp.MustCompile(`\b[A-Z]{1,2}\d{6,9}\b`), "[ID_DOC]"},
}

func RedactPII(text string) string {
	result := text
	for _, r := range redactionPatterns {
		result = r.pattern.ReplaceAllString(result, r.replace)
	}
	return result
}
