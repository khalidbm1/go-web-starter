package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func call(t *testing.T, h http.Handler, method, path, body string, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method != "GET" {
		req.Header.Set("X-Requested-With", "GoWebStarter")
		req.Header.Set("Content-Type", "application/json")
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	return out
}
func want(t *testing.T, out *httptest.ResponseRecorder, status int) {
	t.Helper()
	if out.Code != status {
		t.Fatalf("want status %d, got %d: %.220s", status, out.Code, out.Body.String())
	}
}
func parsed[T any](t *testing.T, out *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func fixture(t *testing.T, db *sql.DB) *app {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<title>Website fixture</title>"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := newApp(db, "https://example.test", dir)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestPublicSurface(t *testing.T) {
	a := fixture(t, nil)
	h := a.routes()
	for _, path := range []string{"/", "/signup", "/signin", "/app"} {
		out := call(t, h, "GET", path, "", nil, "")
		want(t, out, 200)
		if !strings.Contains(out.Body.String(), "<title>") {
			t.Fatal("missing HTML")
		}
	}
	want(t, call(t, h, "GET", "/api/health", "", nil, ""), 200)
	want(t, call(t, h, "GET", "/api/"+a.collection, "", nil, ""), 401)
	want(t, call(t, h, "POST", "/api/auth/login", "{}", nil, "https://other.test"), 403)
	want(t, call(t, h, "GET", "/api/unknown-route", "", nil, ""), 404)
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	want(t, out, 403)
	if _, err := newApp(nil, "http://example.test", ""); err == nil {
		t.Fatal("expected public cleartext origin to fail")
	}
	if _, err := newApp(nil, "http://localhost:8080", ""); err != nil {
		t.Fatal(err)
	}
	c := a.cookie("test", 60)
	if !c.HttpOnly || !c.Secure || c.Path != "/" || c.SameSite != http.SameSiteLaxMode || !strings.HasPrefix(c.Name, "__Host-") {
		t.Fatal("invalid session attributes")
	}
}
func TestRecordValidation(t *testing.T) {
	for _, body := range []string{`{"title":"  ","status":"active"}`, `{"title":"x","status":"unknown"}`, `{"title":"a\u0000b","status":"done"}`, `{"title":"x","status":"done","owner_id":"client-supplied"}`} {
		out := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if _, ok := recordBody(out, r); ok {
			t.Fatalf("accepted %s", body)
		}
	}
}
func TestPostgresWorkflow(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := migrate(ctx, db); err != nil {
		t.Fatalf("repeated migration: %v", err)
	}
	a := fixture(t, db)
	h := a.routes()
	path := "/api/" + a.collection
	pw := "example-test-passphrase"
	mkUser := func() (string, *http.Cookie) {
		t.Helper()
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		email := "test-" + hex.EncodeToString(b) + "@example.test"
		body, _ := json.Marshal(credentials{Email: email, Password: pw})
		out := call(t, h, "POST", "/api/auth/register", string(body), nil, a.origin)
		want(t, out, 201)
		if got := parsed[user](t, out); got.Email != email || got.ID == "" {
			t.Fatal("registration identity")
		}
		t.Cleanup(func() {
			cleanup, err := sql.Open("postgres", dsn)
			if err != nil {
				t.Error(err)
				return
			}
			defer cleanup.Close()
			if _, err = cleanup.Exec("DELETE FROM users WHERE email=$1", email); err != nil {
				t.Error(err)
			}
		})
		return email, out.Result().Cookies()[0]
	}
	email, owner := mkUser()
	_, other := mkUser()
	want(t, call(t, h, "GET", "/api/auth/me", "", owner, ""), 200)
	want(t, call(t, h, "GET", "/api/ready", "", nil, ""), 200)
	payload := `{"title":"First entry","notes":"Private notes","status":"planned"}`
	out := call(t, h, "POST", path, payload, owner, a.origin)
	want(t, out, 201)
	entry := parsed[record](t, out)
	detail := path + "/" + entry.ID
	if entry.Title != "First entry" || entry.Status != "planned" {
		t.Fatal("creation fields")
	}
	want(t, call(t, h, "GET", detail, "", owner, ""), 200)
	otherList := call(t, h, "GET", path, "", other, "")
	want(t, otherList, 200)
	if p := parsed[struct{ Items []record }](t, otherList); len(p.Items) != 0 {
		t.Fatal("unexpected records for second account")
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		want(t, call(t, h, method, detail, payload, other, a.origin), 404)
	}
	want(t, call(t, h, "PUT", detail, `{"title":" ","notes":"","status":"active"}`, owner, a.origin), 422)
	updated := call(t, h, "PUT", detail, `{"title":"Revised","notes":"Updated notes","status":"done"}`, owner, a.origin)
	want(t, updated, 200)
	if item := parsed[record](t, updated); item.Title != "Revised" || item.Notes != "Updated notes" || item.Status != "done" {
		t.Fatal("update fields")
	}
	want(t, call(t, h, "POST", "/api/auth/logout", "", owner, a.origin), 204)
	want(t, call(t, h, "GET", detail, "", owner, ""), 401)
	login := func(pass string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(credentials{Email: email, Password: pass})
		return call(t, h, "POST", "/api/auth/login", string(body), nil, a.origin)
	}
	want(t, login("wrong-example-password"), 401)
	signed := login(pw)
	want(t, signed, 200)
	owner = signed.Result().Cookies()[0]
	restarted := fixture(t, db).routes()
	read := call(t, restarted, "GET", detail, "", owner, "")
	want(t, read, 200)
	if parsed[record](t, read).Title != "Revised" {
		t.Fatal("value after restart")
	}
	want(t, call(t, h, "DELETE", detail, "", owner, a.origin), 204)
	want(t, call(t, h, "GET", detail, "", owner, ""), 404)
	want(t, call(t, h, "DELETE", detail, "", owner, a.origin), 404)
	want(t, call(t, h, "GET", path+"?offset=-1", "", owner, ""), 400)
}
