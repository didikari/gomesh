package auth

import (
	"testing"
	"time"
)

func TestTOTPGenerationAndValidation(t *testing.T) {
	// 1. Generate new secret
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("failed generating secret: %v", err)
	}
	if len(secret) < 16 {
		t.Fatalf("secret too short: %s", secret)
	}

	// 2. Compute current code
	now := time.Now()
	code, err := ComputeTOTP(secret, now)
	if err != nil {
		t.Fatalf("failed computing TOTP: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("expected 6-digit code, got %s", code)
	}

	// 3. Validate code immediately
	if !ValidateTOTP(secret, code) {
		t.Fatal("expected current TOTP code to validate successfully")
	}

	// 4. Validate with 30s skew tolerance
	codePrev, _ := ComputeTOTP(secret, now.Add(-25*time.Second))
	if !ValidateTOTP(secret, codePrev) {
		t.Fatal("expected code within 30s skew to validate")
	}

	// 5. Wrong code must fail
	if ValidateTOTP(secret, "000000") && code != "000000" {
		t.Fatal("expected wrong code to fail")
	}

	// 6. Expired code (> 2 minutes ago) must fail
	codeOld, _ := ComputeTOTP(secret, now.Add(-120*time.Second))
	if ValidateTOTP(secret, codeOld) {
		t.Fatal("expected 2-minute old code to fail validation")
	}

	// 7. OTPAuth URL generation
	otpURL := GenerateOTPAuthURL("GoMesh", "admin", secret)
	if otpURL == "" {
		t.Fatal("expected non-empty otpauth URL")
	}
}
