package main

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

type profileView struct {
	user
	Bio string `json:"bio"`
}
type profileEdit struct {
	Email           string `json:"email"`
	DisplayName     string `json:"displayName"`
	Bio             string `json:"bio"`
	CurrentPassword string `json:"currentPassword"`
}

func (a *app) profile(w http.ResponseWriter, r *http.Request, u user) {
	var v profileView
	err := a.db.QueryRowContext(r.Context(), "SELECT id,email,role,display_name,bio FROM users WHERE id=$1 AND active", u.ID).Scan(&v.ID, &v.Email, &v.Role, &v.DisplayName, &v.Bio)
	if err != nil {
		accountError(w, err)
		return
	}
	jsonResponse(w, 200, v)
}

func (a *app) allowCredentialCheck(w http.ResponseWriter, r *http.Request, id, current string) bool {
	if current == "" || len(current) > 512 {
		problem(w, 403, "Enter your current password.")
		return false
	}
	ok, err := a.withinLimit(r.Context(), "profile:"+id, 20)
	if err != nil {
		dbError(w, err)
		return false
	}
	if !ok {
		w.Header().Set("Retry-After", "600")
		problem(w, 429, "Too many attempts. Try again later.")
		return false
	}
	return a.takeHashSlot(w)
}

type ownAccount struct {
	email      string
	salt, hash []byte
}

func (a *app) lockProfile(w http.ResponseWriter, r *http.Request, u user) (*sql.Tx, ownAccount, bool) {
	var current ownAccount
	ctx := r.Context()
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		dbError(w, err)
		return nil, current, false
	}
	err = tx.QueryRowContext(ctx, `SELECT u.email,u.salt,u.password_hash FROM users u JOIN sessions s ON s.user_id=u.id
		WHERE u.id=$1 AND u.active AND s.token_hash=$2 AND s.expires_at>now() FOR UPDATE OF u`, u.ID, a.tokenHash(r)).Scan(&current.email, &current.salt, &current.hash)
	if err != nil {
		tx.Rollback()
		if errors.Is(err, sql.ErrNoRows) {
			problem(w, 401, "Sign in to continue.")
		} else {
			dbError(w, err)
		}
		return nil, current, false
	}
	return tx, current, true
}
func matchesCurrent(w http.ResponseWriter, entered string, stored ownAccount) bool {
	if subtle.ConstantTimeCompare(passwordHash(entered, stored.salt), stored.hash) != 1 {
		problem(w, 403, "The current password is incorrect.")
		return false
	}
	return true
}
func (a *app) finishCredentialChange(w http.ResponseWriter, r *http.Request, tx *sql.Tx, id string) {
	if _, err := tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE user_id=$1", id); err != nil {
		dbError(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		dbError(w, err)
		return
	}
	http.SetCookie(w, a.cookie("", -1))
	jsonResponse(w, 200, map[string]bool{"signInRequired": true})
}
func (a *app) editProfile(w http.ResponseWriter, r *http.Request, u user) {
	var v profileEdit
	if !decode(w, r, &v) {
		return
	}
	v.Email = normalizedEmail(v.Email)
	v.DisplayName = strings.TrimSpace(v.DisplayName)
	v.Bio = strings.TrimSpace(v.Bio)
	if !validEmail(v.Email) || !textFits(v.DisplayName, 80) || !textFits(v.Bio, 500) {
		problem(w, 422, "Use a valid email, a name up to 80 characters, and a bio up to 500 characters.")
		return
	}
	changingEmail := v.Email != u.Email
	if changingEmail {
		if !a.allowCredentialCheck(w, r, u.ID, v.CurrentPassword) {
			return
		}
		defer func() { <-a.hashSlots }()
	}
	tx, stored, ok := a.lockProfile(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback()
	if stored.email != u.Email {
		problem(w, 409, "Your email changed. Reload your profile.")
		return
	}
	if changingEmail && !matchesCurrent(w, v.CurrentPassword, stored) {
		return
	}
	if _, err := tx.ExecContext(r.Context(), "UPDATE users SET email=$1,display_name=$2,bio=$3 WHERE id=$4", v.Email, v.DisplayName, v.Bio, u.ID); err != nil {
		accountError(w, err)
		return
	}
	if changingEmail {
		a.finishCredentialChange(w, r, tx, u.ID)
		return
	}
	if err := tx.Commit(); err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, map[string]bool{"signInRequired": false})
}
func (a *app) changePassword(w http.ResponseWriter, r *http.Request, u user) {
	var v struct {
		Current string `json:"currentPassword"`
		Next    string `json:"newPassword"`
	}
	if !decode(w, r, &v) {
		return
	}
	if !validPassword(v.Next) {
		problem(w, 422, "Use a new password of at least 12 characters (up to 512 bytes).")
		return
	}
	if !a.allowCredentialCheck(w, r, u.ID, v.Current) {
		return
	}
	defer func() { <-a.hashSlots }()
	tx, stored, ok := a.lockProfile(w, r, u)
	if !ok {
		return
	}
	defer tx.Rollback()
	if !matchesCurrent(w, v.Current, stored) {
		return
	}
	salt, hash := freshPassword(v.Next)
	if _, err := tx.ExecContext(r.Context(), "UPDATE users SET salt=$1,password_hash=$2 WHERE id=$3", salt, hash, u.ID); err != nil {
		dbError(w, err)
		return
	}
	a.finishCredentialChange(w, r, tx, u.ID)
}
