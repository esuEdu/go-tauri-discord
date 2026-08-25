package files

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"time"
)

const signingPurpose = "vocalis-attachment-urls"

type Signer struct {
	key []byte
	ttl time.Duration
}

const defaultURLTTL = 24 * time.Hour

func NewSigner(secret []byte, ttl time.Duration) *Signer {
	if ttl <= 0 {
		ttl = defaultURLTTL
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingPurpose))
	return &Signer{key: mac.Sum(nil), ttl: ttl}
}

func (s *Signer) SignedURL(path string) string {
	if s == nil {
		return path
	}
	expiry := strconv.FormatInt(time.Now().Add(s.ttl).Unix(), 10)
	return path + "?exp=" + expiry + "&sig=" + s.sign(path, expiry)
}

func (s *Signer) Allows(path, expiry, signature string) bool {
	if s == nil {
		return false
	}

	seconds, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || time.Now().After(time.Unix(seconds, 0)) {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(s.sign(path, expiry)))
}

func (s *Signer) sign(path, expiry string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(expiry))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
