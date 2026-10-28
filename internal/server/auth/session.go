package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"gomesh/internal/server/store"
)

const (
	SessionCookieName = "gomesh_session"
	DefaultSessionTTL = 24 * time.Hour
)

type contextKey string

const SessionContextKey contextKey = "gomesh_session"

// Session represents an authenticated web dashboard session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// SessionManager manages active user sessions in memory
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	ttl      time.Duration
}

// NewSessionManager creates a new thread-safe session manager
func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	sm := &SessionManager{
		sessions: make(map[string]*Session),
		ttl:      ttl,
	}

	// Background routine to purge expired sessions every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			sm.purgeExpired()
		}
	}()

	return sm
}

// generateSecureToken creates a cryptographically secure random session ID
func generateSecureToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Create creates and stores a new session for a user
func (sm *SessionManager) Create(user *store.User) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sessionID := generateSecureToken()
	now := time.Now().UTC()
	sess := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: now,
		ExpiresAt: now.Add(sm.ttl),
	}
	sm.sessions[sessionID] = sess
	return sess
}

// Get retrieves a session if it exists and has not expired
func (sm *SessionManager) Get(sessionID string) (*Session, bool) {
	if sessionID == "" {
		return nil, false
	}
	sm.mu.RLock()
	sess, exists := sm.sessions[sessionID]
	sm.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		sm.Revoke(sessionID)
		return nil, false
	}

	return sess, true
}

// Revoke deletes a session
func (sm *SessionManager) Revoke(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

// purgeExpired removes expired sessions
func (sm *SessionManager) purgeExpired() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	now := time.Now().UTC()
	for id, sess := range sm.sessions {
		if now.After(sess.ExpiresAt) {
			delete(sm.sessions, id)
		}
	}
}

// SetCookie writes session cookie to response
func SetCookie(w http.ResponseWriter, sessionID string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearCookie removes session cookie from client
func ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// GetSessionFromRequest extracts session from cookie or Authorization header
func (sm *SessionManager) GetSessionFromRequest(r *http.Request) (*Session, bool) {
	// 1. Try Cookie
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		if sess, ok := sm.Get(cookie.Value); ok {
			return sess, true
		}
	}

	// 2. Try Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token := authHeader[7:]
		if sess, ok := sm.Get(token); ok {
			return sess, true
		}
	}

	// 3. Try query param token (helpful for WebSocket browser handshakes)
	tokenParam := r.URL.Query().Get("token")
	if tokenParam != "" {
		if sess, ok := sm.Get(tokenParam); ok {
			return sess, true
		}
	}

	return nil, false
}

// RequireAuth middleware verifies that the incoming request has a valid session
func (sm *SessionManager) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sm.GetSessionFromRequest(r)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), SessionContextKey, sess)
		next(w, r.WithContext(ctx))
	}
}

// RequireAdmin middleware ensures the user is an admin
func (sm *SessionManager) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := sm.GetSessionFromRequest(r)
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		if sess.Role != "admin" {
			http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), SessionContextKey, sess)
		next(w, r.WithContext(ctx))
	}
}

// FromContext extracts current user session from context
func FromContext(ctx context.Context) (*Session, bool) {
	sess, ok := ctx.Value(SessionContextKey).(*Session)
	return sess, ok
}
