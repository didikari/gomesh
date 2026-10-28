package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GenerateTOTPSecret creates a 20-byte random secret encoded in Base32 (no padding)
func GenerateTOTPSecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed generating random bytes: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// ComputeTOTP calculates the 6-digit TOTP code for a given secret and timestamp
func ComputeTOTP(secret string, t time.Time) (string, error) {
	cleanSecret := strings.ToUpper(strings.TrimSpace(secret))
	cleanSecret = strings.ReplaceAll(cleanSecret, " ", "")

	// Decode Base32
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleanSecret)
	if err != nil {
		// Fallback try with padding
		key, err = base32.StdEncoding.DecodeString(cleanSecret)
		if err != nil {
			return "", fmt.Errorf("invalid base32 secret: %w", err)
		}
	}

	// 30-second time step counter
	counter := uint64(t.Unix() / 30)

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	// HMAC-SHA1
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	hash := mac.Sum(nil)

	// Dynamic Truncation (RFC 4226)
	offset := hash[len(hash)-1] & 0x0f
	binaryCode := (int(hash[offset])&0x7f)<<24 |
		(int(hash[offset+1])&0xff)<<16 |
		(int(hash[offset+2])&0xff)<<8 |
		(int(hash[offset+3]) & 0xff)

	otp := binaryCode % 1000000
	return fmt.Sprintf("%06d", otp), nil
}

// ValidateTOTP verifies a 6-digit code against a secret with +/- 1 time step tolerance (skew)
func ValidateTOTP(secret, code string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}

	now := time.Now()
	// Check previous step (-30s), current step (0s), and next step (+30s)
	steps := []time.Time{
		now.Add(-30 * time.Second),
		now,
		now.Add(30 * time.Second),
	}

	for _, stepTime := range steps {
		expected, err := ComputeTOTP(secret, stepTime)
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(expected)) == 1 {
			return true
		}
	}

	return false
}

// GenerateOTPAuthURL creates the standard otpauth URI recognized by Google Authenticator / Authy
func GenerateOTPAuthURL(issuer, account, secret string) string {
	cleanSecret := strings.ToUpper(strings.TrimSpace(secret))
	cleanSecret = strings.ReplaceAll(cleanSecret, " ", "")

	label := url.PathEscape(fmt.Sprintf("%s:%s", issuer, account))
	params := url.Values{}
	params.Set("secret", cleanSecret)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", "6")
	params.Set("period", "30")

	return fmt.Sprintf("otpauth://totp/%s?%s", label, params.Encode())
}
