package main

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type recordInput struct {
	Title  string `json:"title"`
	Notes  string `json:"notes"`
	Status string `json:"status"`
}
type record struct {
	ID string `json:"id"`
	recordInput
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const recordCols = "id,title,notes,status,created_at,updated_at"

var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func recordBody(w http.ResponseWriter, r *http.Request) (recordInput, bool) {
	var item recordInput
	if !decode(w, r, &item) {
		return item, false
	}
	item.Title = strings.TrimSpace(item.Title)
	if item.Title == "" || utf8.RuneCountInString(item.Title) > 160 || utf8.RuneCountInString(item.Notes) > 4000 ||
		strings.ContainsRune(item.Title, 0) || strings.ContainsRune(item.Notes, 0) ||
		(item.Status != "planned" && item.Status != "active" && item.Status != "done") {
		problem(w, 422, "Use a title of 1–160 characters, up to 4,000 characters of notes, and a valid status.")
		return item, false
	}
	return item, true
}
func scanRecord(row interface{ Scan(...any) error }) (record, error) {
	var item record
	err := row.Scan(&item.ID, &item.Title, &item.Notes, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}
func sendRecord(w http.ResponseWriter, row *sql.Row, status int) {
	item, err := scanRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Record not found.")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, status, item)
}
func key(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		problem(w, 404, "Record not found.")
		return "", false
	}
	return id, true
}
func (a *app) listRecords(w http.ResponseWriter, r *http.Request, u user) {
	const size = 25
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > 2147483647 {
			problem(w, 400, "Use a nonnegative offset up to 2147483647.")
			return
		}
		offset = n
	}
	rows, err := a.db.QueryContext(r.Context(), "SELECT "+recordCols+" FROM records WHERE owner_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3", u.ID, size+1, offset)
	if err != nil {
		dbError(w, err)
		return
	}
	defer rows.Close()
	items := []record{}
	for rows.Next() {
		item, err := scanRecord(rows)
		if err != nil {
			dbError(w, err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		dbError(w, err)
		return
	}
	more := len(items) > size
	if more {
		items = items[:size]
	}
	jsonResponse(w, 200, struct {
		Items []record `json:"items"`
		More  bool     `json:"more"`
	}{items, more})
}
func (a *app) createRecord(w http.ResponseWriter, r *http.Request, u user) {
	item, ok := recordBody(w, r)
	if !ok {
		return
	}
	row := a.db.QueryRowContext(r.Context(), "INSERT INTO records(owner_id,title,notes,status) VALUES($1,$2,$3,$4) RETURNING "+recordCols, u.ID, item.Title, item.Notes, item.Status)
	sendRecord(w, row, 201)
}
func (a *app) getRecord(w http.ResponseWriter, r *http.Request, u user) {
	id, ok := key(w, r)
	if !ok {
		return
	}
	sendRecord(w, a.db.QueryRowContext(r.Context(), "SELECT "+recordCols+" FROM records WHERE owner_id=$1 AND id=$2", u.ID, id), 200)
}
func (a *app) updateRecord(w http.ResponseWriter, r *http.Request, u user) {
	id, ok := key(w, r)
	if !ok {
		return
	}
	item, ok := recordBody(w, r)
	if !ok {
		return
	}
	row := a.db.QueryRowContext(r.Context(), "UPDATE records SET title=$1,notes=$2,status=$3,updated_at=now() WHERE owner_id=$4 AND id=$5 RETURNING "+recordCols, item.Title, item.Notes, item.Status, u.ID, id)
	sendRecord(w, row, 200)
}
func (a *app) deleteRecord(w http.ResponseWriter, r *http.Request, u user) {
	id, ok := key(w, r)
	if !ok {
		return
	}
	var deleted string
	err := a.db.QueryRowContext(r.Context(), "DELETE FROM records WHERE owner_id=$1 AND id=$2 RETURNING id", u.ID, id).Scan(&deleted)
	if errors.Is(err, sql.ErrNoRows) {
		problem(w, 404, "Record not found.")
		return
	}
	if err != nil {
		dbError(w, err)
		return
	}
	w.WriteHeader(204)
}
