package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gomesh/internal/server/store"
)

func TestSessionManagerLifecycle(t *testing.T) {
	sm := NewSessionManager(500 * time.Millisecond)

	user := &store.User{
		ID:       "usr-1",
		Username: "admin",
		Role:     "admin",
	}

	sess := sm.Create(user)
	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Retrieve session
	retrieved, ok := sm.Get(sess.ID)
	if !ok || retrieved.Username != "admin" {
		t.Fatalf("expected to retrieve session for admin, got ok=%v", ok)
	}

	// Revoke session
	sm.Revoke(sess.ID)
	_, ok = sm.Get(sess.ID)
	if ok {
		t.Fatal("expected session to be revoked")
	}

	// Test expiration
	sess2 := sm.Create(user)
	time.Sleep(600 * time.Millisecond)
	_, ok = sm.Get(sess2.ID)
	if ok {
		t.Fatal("expected session to be expired")
	}
}

func TestAuthMiddleware(t *testing.T) {
	sm := NewSessionManager(time.Hour)

	userAdmin := &store.User{ID: "u1", Username: "superadmin", Role: "admin"}
	userOperator := &store.User{ID: "u2", Username: "tech1", Role: "operator"}

	sessAdmin := sm.Create(userAdmin)
	sessOp := sm.Create(userOperator)

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := FromContext(r.Context())
		w.Write([]byte("hello " + sess.Username))
	})

	protectedHandler := sm.RequireAuth(dummyHandler)
	adminOnlyHandler := sm.RequireAdmin(dummyHandler)

	// 1. Unauthenticated request -> 401
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	rec1 := httptest.NewRecorder()
	protectedHandler(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", rec1.Code)
	}

	// 2. Authenticated with cookie
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessOp.ID})
	rec2 := httptest.NewRecorder()
	protectedHandler(rec2, req2)
	if rec2.Code != http.StatusOK || rec2.Body.String() != "hello tech1" {
		t.Fatalf("expected 200 OK and 'hello tech1', got %d %q", rec2.Code, rec2.Body.String())
	}

	// 3. Operator accessing AdminOnly -> 403
	req3 := httptest.NewRequest("GET", "/api/admin", nil)
	req3.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessOp.ID})
	rec3 := httptest.NewRecorder()
	adminOnlyHandler(rec3, req3)
	if rec3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", rec3.Code)
	}

	// 4. Admin accessing AdminOnly -> 200 OK
	req4 := httptest.NewRequest("GET", "/api/admin", nil)
	req4.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sessAdmin.ID})
	rec4 := httptest.NewRecorder()
	adminOnlyHandler(rec4, req4)
	if rec4.Code != http.StatusOK || rec4.Body.String() != "hello superadmin" {
		t.Fatalf("expected 200 OK and 'hello superadmin', got %d %q", rec4.Code, rec4.Body.String())
	}
}
