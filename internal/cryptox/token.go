package cryptox

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// comparePepper is not a secret. It only makes HMAC-SHA256 digests a fixed
// 32 bytes so hmac.Equal does not branch on token length.
var comparePepper = []byte("qllm-bearer-compare-v1")

// digest implements runtime behavior for this package.
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

// HMACSHA256B64 returns raw-URL Base64 of HMAC-SHA256(secret, message).
func HMACSHA256B64(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// HMACSHA256Equal reports whether sig is HMAC-SHA256(secret, message).
func HMACSHA256Equal(secret, message, sig string) bool {
	return HMACEqual(HMACSHA256B64(secret, message), sig)
}
