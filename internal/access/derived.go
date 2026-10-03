package access

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"qLLM/internal/cryptox"
	"qLLM/internal/protocol"
)

const maxScopeValueLen = 128

var (
	scopeValueRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	derivedKeyRe = regexp.MustCompile(`^([a-z][a-z0-9_-]*)\.([A-Za-z0-9_-]{1,128})\.([0-9]+)\.([A-Za-z0-9_-]+)$`)
)

type derivedKey struct {
	App    string
	Value  string
	Expiry int64
	HMAC   string
}

// MintDerivedKey builds app.value.expiry.hmac for a template app.
func MintDerivedKey(app, scopeValue string, expiry time.Time, secret string) (string, *protocol.ProtocolError) {
	if !derivedAppName.MatchString(app) {
		return "", protocol.NewError(protocol.ErrConfigError, "invalid app name for derived key", map[string]any{"app": app})
	}
	if err := checkScopeValue(scopeValue); err != nil {
		return "", err
	}
	exp := expiry.Unix()
	msg := derivedMessage(app, scopeValue, exp)
	return msg + "." + cryptox.HMACSHA256B64(secret, msg), nil
}

func checkScopeValue(v string) *protocol.ProtocolError {
	if v == "" || len(v) > maxScopeValueLen || !scopeValueRe.MatchString(v) {
		return protocol.NewError(protocol.ErrForbiddenScope,
			"scope value must match [A-Za-z0-9_-] and be at most 128 characters",
			map[string]any{"scope": v})
	}
	return nil
}

func derivedMessage(app, value string, expiry int64) string {
	return fmt.Sprintf("%s.%s.%d", app, value, expiry)
}

func parseDerivedKey(token string) (derivedKey, bool) {
	m := derivedKeyRe.FindStringSubmatch(token)
	if m == nil {
		return derivedKey{}, false
	}
	exp, err := strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return derivedKey{}, false
	}
	return derivedKey{App: m[1], Value: m[2], Expiry: exp, HMAC: m[4]}, true
}

func verifyDerived(secret string, d derivedKey) bool {
	if time.Now().Unix() >= d.Expiry {
		return false
	}
	msg := derivedMessage(d.App, d.Value, d.Expiry)
	return cryptox.HMACSHA256Equal(secret, msg, d.HMAC)
}

// WithStdioScope returns a copy with QLLM_SCOPE / --scope applied to a template app.
func (a *App) WithStdioScope(raw string) (*App, *protocol.ProtocolError) {
	if a == nil {
		return nil, nil
	}
	out := a.clone()
	if out.ScopeField == "" {
		return out, nil
	}
	vals, err := ParseStdioScope(raw, out.ScopeField)
	if err != nil {
		return nil, err
	}
	if out.ScopeValues == nil {
		out.ScopeValues = map[string]string{}
	}
	for k, v := range vals {
		out.ScopeValues[k] = v
	}
	return out, nil
}

// ParseStdioScope reads QLLM_SCOPE / --scope: "42" or "user_id=42" (comma pairs).
func ParseStdioScope(raw, field string) (map[string]string, *protocol.ProtocolError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	if !strings.Contains(raw, "=") {
		if err := checkScopeValue(raw); err != nil {
			return nil, err
		}
		if field == "" {
			return nil, protocol.NewError(protocol.ErrForbiddenScope,
				"QLLM_SCOPE needs key=value when the app has no scope.field", nil)
		}
		out[field] = raw
		return out, nil
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return nil, protocol.NewError(protocol.ErrForbiddenScope, "invalid QLLM_SCOPE pair", map[string]any{"part": part})
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if err := checkScopeValue(v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}
