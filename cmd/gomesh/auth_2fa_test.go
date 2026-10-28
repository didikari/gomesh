package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gomesh/internal/server/auth"
	"gomesh/internal/server/hub"
	"gomesh/internal/server/store"
)

func TestTwoFactorAuthenticationFlow(t *testing.T) {
	// Initialize in-memory store
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer st.Close()

	if _, _, err := st.EnsureDefaultAdmin(context.Background()); err != nil {
		t.Fatalf("failed to ensure admin: %v", err)
	}

	sm := auth.NewSessionManager(time.Hour)
	h := hub.New(st)
	defer h.Stop()

	handler := buildServerMux(st, sm, h)

	// Helper to make requests
	doRequest := func(method, path string, body interface{}, cookie *http.Cookie) (*httptest.ResponseRecorder, map[string]interface{}) {
		var reqBody []byte
		if body != nil {
			reqBody, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(reqBody))
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		var respData map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &respData)
		return rec, respData
	}

	// 1. Initial Login (Default credentials admin/admin123, 2FA disabled)
	rec, resp := doRequest("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: code %d, body %s", rec.Code, rec.Body.String())
	}
	if resp["requires_2fa"] == true {
		t.Fatalf("expected 2fa to be disabled initially")
	}

	// Extract session cookie
	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected session cookie to be set")
	}

	// 2. Check 2FA Status (should be false)
	rec, resp = doRequest("GET", "/api/auth/2fa/status", nil, sessionCookie)
	if rec.Code != http.StatusOK || resp["enabled"] != false {
		t.Fatalf("expected 2fa enabled=false, got code %d, resp: %v", rec.Code, resp)
	}

	// 3. Initiate 2FA Setup
	rec, resp = doRequest("POST", "/api/auth/2fa/setup", nil, sessionCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup failed: code %d, body %s", rec.Code, rec.Body.String())
	}
	secret, ok := resp["secret"].(string)
	if !ok || len(secret) == 0 {
		t.Fatalf("expected non-empty secret in setup response")
	}
	qrURL, ok := resp["qr_url"].(string)
	if !ok || !strings.HasPrefix(qrURL, "data:image/png;base64,") {
		t.Fatalf("expected qr_url to be base64 data URI, got: %s", qrURL)
	}

	// 4. Try enabling 2FA with invalid code
	rec, _ = doRequest("POST", "/api/auth/2fa/enable", map[string]string{
		"secret": secret,
		"code":   "000000",
	}, sessionCookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid TOTP code, got %d", rec.Code)
	}

	// 5. Enable 2FA with valid TOTP code
	validCode, err := auth.ComputeTOTP(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to compute totp: %v", err)
	}

	rec, resp = doRequest("POST", "/api/auth/2fa/enable", map[string]string{
		"secret": secret,
		"code":   validCode,
	}, sessionCookie)
	if rec.Code != http.StatusOK || resp["success"] != true {
		t.Fatalf("enable failed: code %d, body %s", rec.Code, rec.Body.String())
	}

	// Verify 2FA status is now enabled
	rec, resp = doRequest("GET", "/api/auth/2fa/status", nil, sessionCookie)
	if rec.Code != http.StatusOK || resp["enabled"] != true {
		t.Fatalf("expected 2fa enabled=true, got %v", resp)
	}

	// 6. Logout
	doRequest("POST", "/api/auth/logout", nil, sessionCookie)

	// 7. Login again with username & password -> Must prompt for 2FA
	rec, resp = doRequest("POST", "/api/auth/login", map[string]string{
		"username": "admin",
		"password": "admin123",
	}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on initial step of 2FA login, got %d", rec.Code)
	}
	if resp["requires_2fa"] != true {
		t.Fatalf("expected requires_2fa=true, got %v", resp)
	}
	tempToken, ok := resp["temp_token"].(string)
	if !ok || tempToken == "" {
		t.Fatalf("expected temp_token in 2fa challenge response")
	}

	// Make sure no session cookie was granted yet
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			t.Fatalf("session cookie should not be granted before 2FA verification")
		}
	}

	// 8. Try completing 2FA login with bad code
	rec, _ = doRequest("POST", "/api/auth/login/2fa", map[string]string{
		"temp_token": tempToken,
		"code":       "999999",
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for bad TOTP code, got %d", rec.Code)
	}

	// 9. Complete 2FA login with valid code
	validCode, _ = auth.ComputeTOTP(secret, time.Now())
	rec, resp = doRequest("POST", "/api/auth/login/2fa", map[string]string{
		"temp_token": tempToken,
		"code":       validCode,
	}, nil)
	if rec.Code != http.StatusOK || resp["success"] != true {
		t.Fatalf("expected 2fa login to succeed, got code %d, body %s", rec.Code, rec.Body.String())
	}

	// Retrieve new authenticated session cookie
	var newSessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionCookieName && c.Value != "" {
			newSessionCookie = c
			break
		}
	}
	if newSessionCookie == nil {
		t.Fatalf("expected valid session cookie after 2FA login")
	}

	// 10. Access protected /api/auth/me
	rec, resp = doRequest("GET", "/api/auth/me", nil, newSessionCookie)
	if rec.Code != http.StatusOK || resp["authenticated"] != true {
		t.Fatalf("expected authenticated=true, got code %d, resp %v", rec.Code, resp)
	}

	// 11. Disable 2FA with wrong password
	rec, _ = doRequest("POST", "/api/auth/2fa/disable", map[string]string{
		"password": "wrongpassword",
	}, newSessionCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong password on disable 2fa, got %d", rec.Code)
	}

	// 12. Disable 2FA with correct password
	rec, resp = doRequest("POST", "/api/auth/2fa/disable", map[string]string{
		"password": "admin123",
	}, newSessionCookie)
	if rec.Code != http.StatusOK || resp["success"] != true {
		t.Fatalf("expected disable 2fa to succeed, got %d, body: %s", rec.Code, rec.Body.String())
	}

	// Check status again
	rec, resp = doRequest("GET", "/api/auth/2fa/status", nil, newSessionCookie)
	if rec.Code != http.StatusOK || resp["enabled"] != false {
		t.Fatalf("expected 2fa enabled=false after disabling, got %v", resp)
	}
}
