package main

import "net/http"

type recordCounts struct {
	Total   int `json:"total"`
	Planned int `json:"planned"`
	Active  int `json:"active"`
	Done    int `json:"done"`
}

func (a *app) dashboard(w http.ResponseWriter, r *http.Request, u user) {
	key, ok := a.dashboardKey(r.URL.Query().Get("module"))
	if !ok {
		problem(w, 404, "Unknown module.")
		return
	}
	var c recordCounts
	err := a.db.QueryRowContext(r.Context(), `SELECT count(*),count(*) FILTER (WHERE status='planned'),
		count(*) FILTER (WHERE status='active'),count(*) FILTER (WHERE status='done')
		FROM records WHERE owner_id=$1 AND resource_key=$2`, u.ID, key).Scan(&c.Total, &c.Planned, &c.Active, &c.Done)
	if err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, c)
}

func (a *app) adminOverview(w http.ResponseWriter, r *http.Request, _ user) {
	var out struct {
		Users    int `json:"users"`
		Active   int `json:"active"`
		Disabled int `json:"disabled"`
		Admins   int `json:"admins"`
		Records  int `json:"records"`
	}
	err := a.db.QueryRowContext(r.Context(), `SELECT count(*),count(*) FILTER (WHERE active),
		count(*) FILTER (WHERE NOT active),count(*) FILTER (WHERE active AND role='admin'),
		(SELECT count(*) FROM records) FROM users`).Scan(&out.Users, &out.Active, &out.Disabled, &out.Admins, &out.Records)
	if err != nil {
		dbError(w, err)
		return
	}
	jsonResponse(w, 200, out)
}
