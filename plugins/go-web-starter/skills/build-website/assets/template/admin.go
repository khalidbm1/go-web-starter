package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
)

type account struct {
	user
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}

const accountColumns = "id,email,role,display_name,active,created_at"

type scanner interface{ Scan(...any) error }

func readAccount(row scanner) (account, error) {
	var v account
	err := row.Scan(&v.ID, &v.Email, &v.Role, &v.DisplayName, &v.Active, &v.CreatedAt)
	return v, err
}

type manageInput struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Active      *bool  `json:"active"`
	Password    string `json:"password"`
}

func textFits(s string, limit int) bool {
	return utf8.RuneCountInString(s) <= limit && !strings.ContainsRune(s, 0)
}
func managementInput(w http.ResponseWriter, r *http.Request, creating bool) (manageInput, bool) {
	var v manageInput
	if !decode(w, r, &v) {
		return v, false
	}
	v.Email = normalizedEmail(v.Email)
	v.DisplayName = strings.TrimSpace(v.DisplayName)
	if !validEmail(v.Email) || !textFits(v.DisplayName, 80) || (v.Role != "user" && v.Role != "admin") || v.Active == nil ||
		((creating || v.Password != "") && !validPassword(v.Password)) {
		problem(w, 422, "Check the email, name, role, account status, and password.")
		return v, false
	}
	return v, true
}
func accountError(w http.ResponseWriter, err error) {
	var pg *pq.Error
	switch {
	case errors.Is(err, sql.ErrNoRows):
		problem(w, 404, "Account not found.")
	case errors.As(err, &pg) && pg.Code == "23505":
		problem(w, 409, "An account already uses this email.")
	default:
		dbError(w, err)
	}
}
func (a *app) adminOnly(next func(http.ResponseWriter, *http.Request, user)) http.HandlerFunc {
	return a.signedIn(func(w http.ResponseWriter, r *http.Request, u user) {
		if u.Role != "admin" {
			problem(w, 403, "Administrator access is required.")
			return
		}
		next(w, r, u)
	})
}

// Serialize changes to administrator access, including the operator command.
func adminLock(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(762881754)")
	return err
}
func (a *app) adminWrite(w http.ResponseWriter, r *http.Request) (*sql.Tx, bool) {
	ctx := r.Context()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		dbError(w, err)
		return nil, false
	}
	if err = adminLock(ctx, tx); err != nil {
		tx.Rollback()
		dbError(w, err)
		return nil, false
	}
	var id string
	err = tx.QueryRowContext(ctx, `SELECT u.id FROM users u JOIN sessions s ON s.user_id=u.id
		WHERE s.token_hash=$1 AND s.expires_at>now() AND u.active AND u.role='admin' FOR SHARE OF u`, a.tokenHash(r)).Scan(&id)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			problem(w, 403, "Administrator access is required.")
		} else {
			dbError(w, err)
		}
		return nil, false
	}
	return tx, true
}

func promote(ctx context.Context, db *sql.DB, email string) error {
	email = normalizedEmail(email)
	if !validEmail(email) {
		return errors.New("supply the email of an existing active account")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = adminLock(ctx, tx); err != nil {
		return err
	}
	var id, role string
	if err = tx.QueryRowContext(ctx, "SELECT id,role FROM users WHERE email=$1 AND active FOR UPDATE", email).Scan(&id, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("create or reactivate the account before promoting it")
		}
		return err
	}
	if role != "admin" {
		if _, err = tx.ExecContext(ctx, "UPDATE users SET role='admin' WHERE id=$1", id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "DELETE FROM sessions WHERE user_id=$1", id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (a *app) users(w http.ResponseWriter, r *http.Request, _ user) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 2147483647 {
			problem(w, 400, "Use a valid nonnegative offset.")
			return
		}
		offset = n
	}
	if !textFits(q, 160) {
		problem(w, 422, "Use a shorter search.")
		return
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT "+accountColumns+` FROM users
		WHERE strpos(lower(email),lower($1))>0 OR strpos(lower(display_name),lower($1))>0
		ORDER BY created_at DESC,id DESC LIMIT 26 OFFSET $2`, q, offset)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	items := []account{}
	for rows.Next() {
		v, err := readAccount(rows)
		if err != nil {
			dbError(w, err)
			return
		}
		items = append(items, v)
	}
	if err = rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	more := len(items) > 25
	if more {
		items = items[:25]
	}
	jsonResponse(w, 200, struct {
		Items []account `json:"items"`
		More  bool      `json:"more"`
	}{items, more})
}
func (a *app) showUser(w http.ResponseWriter, r *http.Request, _ user) {
	id, ok := key(w, r)
	if !ok {
		return
	}
	v, err := readAccount(a.db.QueryRowContext(r.Context(), "SELECT "+accountColumns+" FROM users WHERE id=$1", id))
	if err != nil {
		accountError(w, err)
		return
	}
	jsonResponse(w, 200, v)
}
func (a *app) createUser(w http.ResponseWriter, r *http.Request, _ user) {
	v, ok := managementInput(w, r, true)
	if !ok {
		return
	}
	if !a.takeHashSlot(w) {
		return
	}
	defer func() { <-a.hashSlots }()
	salt, hash := freshPassword(v.Password)
	tx, ok := a.adminWrite(w, r)
	if !ok {
		return
	}
	defer tx.Rollback()
	out, err := readAccount(tx.QueryRowContext(r.Context(), `INSERT INTO users(email,display_name,role,active,salt,password_hash)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING `+accountColumns, v.Email, v.DisplayName, v.Role, *v.Active, salt, hash))
	if err != nil {
		accountError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 201, out)
}
func (a *app) updateUser(w http.ResponseWriter, r *http.Request, actor user) {
	id, ok := key(w, r)
	if !ok {
		return
	}
	v, ok := managementInput(w, r, false)
	if !ok {
		return
	}
	if id == actor.ID && (v.Email != actor.Email || v.Password != "" || v.Role != "admin" || !*v.Active) {
		problem(w, 409, "Use your profile for credentials. Another administrator must change your access.")
		return
	}
	var salt, hash any
	if v.Password != "" {
		if !a.takeHashSlot(w) {
			return
		}
		defer func() { <-a.hashSlots }()
		salt, hash = freshPassword(v.Password)
	}
	tx, ok := a.adminWrite(w, r)
	if !ok {
		return
	}
	defer tx.Rollback()
	old, err := readAccount(tx.QueryRowContext(r.Context(), "SELECT "+accountColumns+" FROM users WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		accountError(w, err)
		return
	}
	out, err := readAccount(tx.QueryRowContext(r.Context(), `UPDATE users SET email=$1,display_name=$2,role=$3,active=$4,
		salt=COALESCE($5,salt),password_hash=COALESCE($6,password_hash) WHERE id=$7 RETURNING `+accountColumns,
		v.Email, v.DisplayName, v.Role, *v.Active, salt, hash, id))
	if err != nil {
		accountError(w, err)
		return
	}
	if old.Role != out.Role || old.Active != out.Active || old.Email != out.Email || v.Password != "" {
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=$1", id); err != nil {
			dbError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, out)
}
func (a *app) deleteUser(w http.ResponseWriter, r *http.Request, actor user) {
	id, ok := key(w, r)
	if !ok {
		return
	}
	if id == actor.ID {
		problem(w, 409, "Another administrator must delete your account.")
		return
	}
	tx, ok := a.adminWrite(w, r)
	if !ok {
		return
	}
	defer tx.Rollback()
	var removed string
	if err := tx.QueryRowContext(r.Context(), "DELETE FROM users WHERE id=$1 RETURNING id", id).Scan(&removed); err != nil {
		accountError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbError(w, err)
		return
	}
	w.WriteHeader(204)
}
