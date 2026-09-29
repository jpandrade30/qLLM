package cryptox

import (
	"crypto/hmac"
	"crypto/sha256"
)

// comparePepper is not a secret. It only makes HMAC-SHA256 digests a fixed
// 32 bytes so hmac.Equal does not branch on token length.
var comparePepper = []byte("qllm-bearer-compare-v1")

func digest(s string) []byte {
	mac := hmac.New(sha256.New, comparePepper)
	_, _ = mac.Write([]byte(s))
	return mac.Sum(nil)
}

// HMACEqual reports whether got and want are the same token without leaking
// length via early return (unlike subtle.ConstantTimeCompare on raw bytes).
func HMACEqual(got, want string) bool {
	return hmac.Equal(digest(got), digest(want))
}
