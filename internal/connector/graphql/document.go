package graphql

import (
	"regexp"
	"strings"
	"unicode"

	"qLLM/internal/protocol"
)

// writeKeywords are rejected in GraphQL documents (defense in depth beyond operation type).
var writeKeywords = []string{
	"mutation", "subscription",
	"insert", "update", "delete", "merge", "truncate",
	"drop", "alter", "create", "copy", "attach",
}

var replaceIntoRE = regexp.MustCompile(`(?i)\breplace\s+into\b`)

// ValidateDocument ensures the preset GraphQL document is a read-only query.
// Comments (#…) and simple string literals are stripped before the scan.
func ValidateDocument(document string) *protocol.ProtocolError {
	doc := strings.TrimSpace(document)
	if doc == "" {
		return protocol.NewError(protocol.ErrConfigError, "graphql document is empty", nil)
	}
	stripped := stripCommentsAndStrings(doc)
	if err := rejectWriteKeywords(stripped); err != nil {
		return err
	}
	if err := requireQueryOperation(stripped); err != nil {
		return err
	}
	return nil
}

func stripCommentsAndStrings(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c == '#' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}
		if c == '"' {
			i++
			for i < len(s) {
				if s[i] == '\\' && i+1 < len(s) {
					i += 2
					continue
				}
				if s[i] == '"' {
					i++
					break
				}
				i++
			}
			b.WriteByte(' ')
			continue
		}
		if c == '\'' {
			// GraphQL uses double quotes; treat single-quoted spans as opaque too.
			i++
			for i < len(s) && s[i] != '\'' {
				if s[i] == '\\' && i+1 < len(s) {
					i += 2
					continue
				}
				i++
			}
			if i < len(s) {
				i++
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

func rejectWriteKeywords(stripped string) *protocol.ProtocolError {
	if replaceIntoRE.MatchString(stripped) {
		return protocol.NewError(protocol.ErrConfigError,
			"graphql document forbids REPLACE INTO", map[string]any{"keyword": "REPLACE INTO"})
	}
	for _, kw := range writeKeywords {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)
		if re.MatchString(stripped) {
			return protocol.NewError(protocol.ErrConfigError,
				"graphql document forbids "+strings.ToUpper(kw)+"; only query is allowed",
				map[string]any{"keyword": strings.ToUpper(kw)})
		}
	}
	return nil
}

func requireQueryOperation(stripped string) *protocol.ProtocolError {
	tok := firstSignificantToken(stripped)
	if tok == "" {
		return protocol.NewError(protocol.ErrConfigError, "graphql document has no operation", nil)
	}
	switch strings.ToLower(tok) {
	case "{", "query":
		return nil
	case "mutation", "subscription":
		return protocol.NewError(protocol.ErrConfigError,
			"graphql document only allows query (got "+strings.ToLower(tok)+")",
			map[string]any{"operation": strings.ToLower(tok)})
	default:
		// Named shorthand without keyword is invalid GraphQL; treat as config error.
		return protocol.NewError(protocol.ErrConfigError,
			"graphql document must start with query or '{'", map[string]any{"token": tok})
	}
}

func firstSignificantToken(s string) string {
	i := 0
	for i < len(s) {
		r := rune(s[i])
		if unicode.IsSpace(r) || r == ',' {
			i++
			continue
		}
		if s[i] == '{' {
			return "{"
		}
		j := i
		for j < len(s) {
			r := rune(s[j])
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
				j++
				continue
			}
			break
		}
		if j > i {
			return s[i:j]
		}
		i++
	}
	return ""
}
