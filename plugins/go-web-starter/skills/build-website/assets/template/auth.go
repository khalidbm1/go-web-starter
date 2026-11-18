package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
	"golang.org/x/crypto/argon2"
)

type user struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	DisplayName string `json:"displayName"`
}
type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *app) cookieName() string {
	if a.secure {
		return "__Host-websession"
	}
	return "websession"
}
func (a *app) cookie(value string, age int) *http.Cookie {
	return &http.Cookie{Name: a.cookieName(), Value: value, Path: "/", HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode, MaxAge: age}
}
func (a *app) tokenHash(r *http.Request) []byte {
	c, err := r.Cookie(a.cookieName())
	if err != nil {
		return nil
	}
	token, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil || len(token) != 32 {
		return nil
	}
	hash := sha256.Sum256(token)
	return hash[:]
}
func (a *app) signedIn(next func(http.ResponseWriter, *http.Request, user)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := a.tokenHash(r)
		if key == nil {
			problem(w, 401, "Sign in to continue.")
			return
		}
		var u user
		err := a.db.QueryRowContext(r.Context(), `SELECT u.id,u.email,u.role,u.display_name FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.active`, key).Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName)
		if errors.Is(err, sql.ErrNoRows) {
			problem(w, 401, "Sign in to continue.")
			return
		}
		if err != nil {
			dbError(w, err)
			return
		}
		next(w, r, u)
	}
}

func normalizedEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func validPassword(s string) bool     { return utf8.RuneCountInString(s) >= 12 && len(s) <= 512 }
func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email && len(email) <= 254 && strings.Contains(email, "@")
}

// A rolling time window keyed separately by email and immediate peer caps expensive hashes.
func (a *app) withinLimit(ctx context.Context, bucket string, max int) (bool, error) {
	key := sha256.Sum256([]byte(bucket))
	var attempts int
	err := a.db.QueryRowContext(ctx, `INSERT INTO auth_windows(bucket,began_at,attempts) VALUES($1,now(),1)
		ON CONFLICT(bucket) DO UPDATE SET
		began_at=CASE WHEN auth_windows.began_at<=now()-interval '10 minutes' THEN now() ELSE auth_windows.began_at END,
		attempts=CASE WHEN auth_windows.began_at<=now()-interval '10 minutes' THEN 1 ELSE LEAST(auth_windows.attempts+1,10000) END
		RETURNING attempts`, key[:]).Scan(&attempts)
	return attempts <= max, err
}

func (a *app) input(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	var c credentials
	if !decode(w, r, &c) {
		return c, false
	}
	c.Email = normalizedEmail(c.Email)
	if !validEmail(c.Email) || !validPassword(c.Password) {
		problem(w, 422, "Use a valid email and a password of at least 12 characters (up to 512 bytes).")
		return c, false
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	for _, b := range []struct {
		key string
		max int
	}{{"ip:" + peer, 120}, {"email:" + c.Email, 20}} {
		ok, err := a.withinLimit(r.Context(), b.key, b.max)
		if err != nil {
			dbError(w, err)
			return c, false
		}
		if !ok {
			w.Header().Set("Retry-After", "600")
			problem(w, 429, "Too many attempts. Try again later.")
			return c, false
		}
	}
	return c, a.takeHashSlot(w)
}

func (a *app) takeHashSlot(w http.ResponseWriter) bool {
	select {
	case a.hashSlots <- struct{}{}:
		return true
	default:
		w.Header().Set("Retry-After", "2")
		problem(w, 429, "Account services are busy. Try again shortly.")
		return false
	}
}

func freshPassword(password string) ([]byte, []byte) {
	salt := make([]byte, 16)
	_, _ = rand.Read(salt)
	return salt, passwordHash(password, salt)
}

func passwordHash(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
}

func (a *app) register(w http.ResponseWriter, r *http.Request) {
	c, ok := a.input(w, r)
	if !ok {
		return
	}
	defer func() { <-a.hashSlots }()
	salt, hash := freshPassword(c.Password)
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback()
	var u user
	err = tx.QueryRowContext(r.Context(), `INSERT INTO users(email,salt,password_hash) VALUES($1,$2,$3) RETURNING id,email,role,display_name`, c.Email, salt, hash).Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName)
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		problem(w, 409, "Unable to create an account with that email.")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	a.issueSession(w, r, tx, u, 201)
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	c, ok := a.input(w, r)
	if !ok {
		return
	}
	defer func() { <-a.hashSlots }()
	tx, err := a.db.BeginTx(r.Context(), nil)
	if err != nil {
		dbError(w, err)
		return
	}
	defer tx.Rollback()
	var u user
	var salt, expected []byte
	err = tx.QueryRowContext(r.Context(), "SELECT id,email,role,display_name,salt,password_hash FROM users WHERE email=$1 AND active FOR SHARE", c.Email).Scan(&u.ID, &u.Email, &u.Role, &u.DisplayName, &salt, &expected)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		dbError(w, err)
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		salt = a.dummySalt
		expected = make([]byte, 32)
	}
	actual := passwordHash(c.Password, salt)
	if subtle.ConstantTimeCompare(actual, expected) != 1 || u.ID == "" {
		problem(w, 401, "Email or password is incorrect.")
		return
	}
	a.issueSession(w, r, tx, u, 200)
}

func (a *app) issueSession(w http.ResponseWriter, r *http.Request, tx *sql.Tx, u user, status int) {
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	hash := sha256.Sum256(token)
	if old := a.tokenHash(r); old != nil {
		if _, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", old); err != nil {
			dbError(w, err)
			return
		}
	}
	if _, err := tx.ExecContext(r.Context(), `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,now()+interval '7 days')`, hash[:], u.ID); err != nil {
		dbError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbError(w, err)
		return
	}
	http.SetCookie(w, a.cookie(base64.RawURLEncoding.EncodeToString(token), 7*24*60*60))
	jsonResponse(w, status, u)
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	if hash := a.tokenHash(r); hash != nil {
		if _, err := a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash=$1", hash); err != nil {
			dbError(w, err)
			return
		}
	}
	http.SetCookie(w, a.cookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func (a *app) prune(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = a.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at<=now()")
			_, _ = a.db.ExecContext(ctx, "DELETE FROM auth_windows WHERE began_at<now()-interval '1 day'")
		}
	}
}
