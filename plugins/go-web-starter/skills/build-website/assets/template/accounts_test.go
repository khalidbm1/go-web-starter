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
	"strings"
	"testing"
)

func TestProfilesAdminAndModules(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable PostgreSQL database")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	a := fixture(t, db)
	if len(a.modules) == 1 {
		a.modules = append(a.modules, module{Resource: "qa-extra", Collection: "qa-extras", storeKey: "qa-extra"})
	}
	h := a.routes()
	primary := "/api/" + a.collection
	extra := "/api/" + a.modules[1].Collection
	do := func(method, path string, body any, c *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		raw := ""
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			raw = string(b)
		}
		return call(t, h, method, path, raw, c, a.origin)
	}
	random := make([]byte, 8)
	_, _ = rand.Read(random)
	prefix := "acct-" + hex.EncodeToString(random)
	mail := func(name string) string { return prefix + "-" + name + "@example.test" }
	ids := []string{}
	defer func() {
		for _, id := range ids {
			if _, err := db.Exec("DELETE FROM users WHERE id=$1", id); err != nil {
				t.Error(err)
			}
		}
	}()
	const pw = "first-test-passphrase"
	creds := func(email, password string) any { return map[string]string{"email": email, "password": password} }
	mk := func(email string) (user, *http.Cookie) {
		t.Helper()
		out := do("POST", "/api/auth/register", creds(email, pw), nil)
		want(t, out, 201)
		u := parsed[user](t, out)
		ids = append(ids, u.ID)
		if u.Role != "user" {
			t.Fatal("public registration assigned elevated access")
		}
		return u, out.Result().Cookies()[0]
	}
	sign := func(email, password string) *http.Cookie {
		t.Helper()
		out := do("POST", "/api/auth/login", creds(email, password), nil)
		want(t, out, 200)
		return out.Result().Cookies()[0]
	}
	admin, initial := mk(mail("admin"))
	member, session := mk(mail("member"))
	for _, path := range []string{"/api/profile", "/api/dashboard", "/api/admin/users", "/api/admin/overview"} {
		want(t, do("GET", path, nil, nil), 401)
	}
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		want(t, do(method, "/api/admin/users/"+admin.ID, nil, session), 403)
	}
	want(t, do("POST", "/api/admin/users", nil, session), 403)
	want(t, do("GET", "/api/admin/overview", nil, session), 403)
	want(t, do("POST", "/api/auth/register", map[string]any{"email": mail("invalid"), "password": pw, "role": "admin"}, nil), 400)
	if err := promote(ctx, db, admin.Email); err != nil {
		t.Fatal(err)
	}
	want(t, do("GET", "/api/auth/me", nil, initial), 401)
	adminSession := sign(admin.Email, pw)
	if err := promote(ctx, db, admin.Email); err != nil {
		t.Fatal(err)
	}
	want(t, do("GET", "/api/admin/overview", nil, adminSession), 200)
	accounts := "/api/admin/users"
	makeAccount := func(email, role string, active bool, password string) map[string]any {
		return map[string]any{
			"email": email, "displayName": "Managed", "role": role, "active": active, "password": password}
	}
	want(t, do("PUT", accounts+"/"+admin.ID, makeAccount(admin.Email, "user", true, ""), adminSession), 409)
	want(t, do("PUT", accounts+"/"+admin.ID, makeAccount(admin.Email, "admin", false, ""), adminSession), 409)
	want(t, do("DELETE", accounts+"/"+admin.ID, nil, adminSession), 409)
	out := do("POST", accounts, makeAccount(mail("third"), "user", true, pw), adminSession)
	want(t, out, 201)
	third := parsed[account](t, out)
	ids = append(ids, third.ID)
	list := do("GET", accounts+"?q="+prefix+"-third", nil, adminSession)
	want(t, list, 200)
	if rows := parsed[struct{ Items []account }](t, list); len(rows.Items) != 1 || rows.Items[0].ID != third.ID {
		t.Fatal("user search did not select the right account")
	}
	want(t, do("GET", accounts+"/"+third.ID, nil, adminSession), 200)
	thirdSession := sign(third.Email, pw)
	want(t, do("PUT", accounts+"/"+third.ID, makeAccount(third.Email, "user", false, ""), adminSession), 200)
	want(t, do("GET", "/api/auth/me", nil, thirdSession), 401)
	want(t, do("POST", "/api/auth/login", creds(third.Email, pw), nil), 401)
	want(t, do("PUT", accounts+"/"+third.ID, makeAccount(third.Email, "admin", true, ""), adminSession), 200)
	thirdSession = sign(third.Email, pw)
	want(t, do("GET", "/api/admin/overview", nil, thirdSession), 200)
	want(t, do("PUT", accounts+"/"+third.ID, makeAccount(third.Email, "user", true, "replacement-safe-password"), adminSession), 200)
	want(t, do("GET", "/api/admin/overview", nil, thirdSession), 401)
	want(t, do("POST", "/api/auth/login", creds(third.Email, pw), nil), 401)
	thirdSession = sign(third.Email, "replacement-safe-password")
	want(t, do("GET", accounts, nil, thirdSession), 403)
	want(t, do("DELETE", accounts+"/"+third.ID, nil, adminSession), 204)
	want(t, do("GET", "/api/profile", nil, thirdSession), 401)

	item := map[string]string{"title": "Private work", "notes": "Test", "status": "active"}
	created := do("POST", primary, item, session)
	want(t, created, 201)
	id := parsed[record](t, created).ID
	want(t, do("GET", extra+"/"+id, nil, session), 404)
	want(t, do("PUT", extra+"/"+id, item, session), 404)
	want(t, do("DELETE", extra+"/"+id, nil, session), 404)
	want(t, do("POST", extra, item, session), 201)
	p := do("GET", extra, nil, session)
	want(t, p, 200)
	if rows := parsed[struct{ Items []record }](t, p); len(rows.Items) != 1 || rows.Items[0].ID == id {
		t.Fatal("module boundaries")
	}
	for _, route := range []string{"/api/dashboard", "/api/dashboard?module=" + a.modules[1].Collection} {
		o := do("GET", route, nil, session)
		want(t, o, 200)
		n := parsed[recordCounts](t, o)
		if n.Total != 1 || n.Active != 1 {
			t.Fatalf("scope counts: %+v", n)
		}
		o = do("GET", route, nil, adminSession)
		want(t, o, 200)
		if parsed[recordCounts](t, o).Total != 0 {
			t.Fatal("dashboard exposed another user's totals")
		}
	}
	profile := func(email, name, bio, current string) any {
		return map[string]string{"email": email, "displayName": name, "bio": bio, "currentPassword": current}
	}
	want(t, do("PUT", "/api/profile", profile(member.Email, "Rana", "Designer", ""), session), 200)
	out = do("GET", "/api/profile", nil, session)
	want(t, out, 200)
	if v := parsed[profileView](t, out); v.DisplayName != "Rana" || v.Bio != "Designer" {
		t.Fatal("profile failed to persist")
	}
	out = do("GET", "/api/profile", nil, adminSession)
	want(t, out, 200)
	if strings.Contains(out.Body.String(), "Designer") {
		t.Fatal("private bio visible to another user")
	}
	want(t, do("PUT", "/api/profile", map[string]any{"email": member.Email, "role": "admin"}, session), 400)
	want(t, do("PUT", "/api/profile", profile(mail("new"), "Rana", "Designer", "wrong"), session), 403)
	want(t, do("PUT", "/api/profile", profile(admin.Email, "Rana", "Designer", pw), session), 409)
	otherSession := sign(member.Email, pw)
	want(t, do("PUT", "/api/profile", profile(mail("new"), "Rana", "Designer", pw), session), 200)
	for _, c := range []*http.Cookie{session, otherSession} {
		want(t, do("GET", "/api/profile", nil, c), 401)
	}
	want(t, do("POST", "/api/auth/login", creds(member.Email, pw), nil), 401)
	session = sign(mail("new"), pw)
	want(t, do("PUT", "/api/profile/password", map[string]string{"currentPassword": "wrong", "newPassword": "another-safe-password"}, session), 403)
	want(t, do("PUT", "/api/profile/password", map[string]string{"currentPassword": pw, "newPassword": "another-safe-password"}, session), 200)
	want(t, do("GET", "/api/profile", nil, session), 401)
	want(t, do("POST", "/api/auth/login", creds(mail("new"), pw), nil), 401)
	fresh := sign(mail("new"), "another-safe-password")
	want(t, do("GET", primary+"/"+id, nil, fresh), 200)
}

func TestModuleRegistry(t *testing.T) {
	for _, config := range []string{
		`{"resource":"book","collection":"profile"}`,
		`{"resource":"book","collection":"books","resources":[{"resource":"note","collection":"books"}]}`,
		`{"resource":"book","collection":"books","resources":[{"resource":"book","collection":"notes"}]}`,
	} {
		if _, err := loadModules([]byte(config)); err == nil {
			t.Fatalf("accepted invalid registry %s", config)
		}
	}
}
