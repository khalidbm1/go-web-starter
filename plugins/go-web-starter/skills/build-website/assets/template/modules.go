package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
)

type module struct {
	Resource   string `json:"resource"`
	Collection string `json:"collection"`
	storeKey   string
}

var slug = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

func validKey(s string) bool {
	switch s {
	case "admin", "auth", "dashboard", "health", "profile", "ready":
		return false
	}
	return len(s) <= 48 && slug.MatchString(s)
}
func loadModules(body []byte) ([]module, error) {
	var config struct {
		Resource   string
		Collection string
		Resources  []module
	}
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, err
	}
	items := append([]module{{Resource: config.Resource, Collection: config.Collection}}, config.Resources...)
	keys, paths := map[string]bool{}, map[string]bool{}
	for i := range items {
		m := &items[i]
		if !validKey(m.Resource) || !validKey(m.Collection) || keys[m.Resource] || paths[m.Collection] {
			return nil, errors.New("invalid or duplicate resource in site.json")
		}
		keys[m.Resource] = true
		paths[m.Collection] = true
		if i > 0 {
			m.storeKey = m.Resource
		}
	}
	return items, nil
}
func (a *app) mountModule(mux *http.ServeMux, m module) {
	root := "/api/" + m.Collection
	register := func(pattern string, action func(http.ResponseWriter, *http.Request, user)) {
		mux.HandleFunc(pattern, a.signedIn(func(w http.ResponseWriter, r *http.Request, u user) {
			r.SetPathValue("resourceKey", m.storeKey)
			action(w, r, u)
		}))
	}
	register("GET "+root, a.listRecords)
	register("POST "+root, a.createRecord)
	register("GET "+root+"/{id}", a.getRecord)
	register("PUT "+root+"/{id}", a.updateRecord)
	register("DELETE "+root+"/{id}", a.deleteRecord)
}
func (a *app) dashboardKey(collection string) (string, bool) {
	if collection == "" {
		return "", true
	}
	for _, m := range a.modules {
		if m.Collection == collection {
			return m.storeKey, true
		}
	}
	return "", false
}
