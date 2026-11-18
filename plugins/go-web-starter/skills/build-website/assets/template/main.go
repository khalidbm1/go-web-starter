package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"

	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"

	"syscall"
	"time"

	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql site.json
var source embed.FS

type app struct {
	modules                       []module
	db                            *sql.DB
	origin, collection, staticDir string
	secure                        bool
	hashSlots                     chan struct{}
	dummySalt                     []byte
}

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(762881753)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (filename text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	files, err := source.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, f := range files {
		var done bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename=$1)", f.Name()).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		body, err := source.ReadFile("migrations/" + f.Name())
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("%s: %w", f.Name(), err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES ($1)", f.Name()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func newApp(db *sql.DB, origin, staticDir string) (*app, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || origin != u.Scheme+"://"+u.Host {
		return nil, errors.New("APP_ORIGIN must be an origin with no path, ending slash, or credentials")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return nil, errors.New("APP_ORIGIN requires HTTPS except on loopback")
	}
	body, err := source.ReadFile("site.json")
	if err != nil {
		return nil, err
	}
	modules, err := loadModules(body)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return &app{db: db, origin: origin, modules: modules, collection: modules[0].Collection, staticDir: staticDir, secure: u.Scheme == "https", hashSlots: make(chan struct{}, 4), dummySalt: salt}, nil
}

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := a.db.PingContext(ctx); err != nil {
			problem(w, 503, "Database unavailable.")
			return
		}
		jsonResponse(w, 200, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("POST /api/auth/register", a.register)
	mux.HandleFunc("POST /api/auth/login", a.login)
	mux.HandleFunc("POST /api/auth/logout", a.logout)
	mux.HandleFunc("GET /api/auth/me", a.signedIn(func(w http.ResponseWriter, r *http.Request, u user) { jsonResponse(w, 200, u) }))
	mux.HandleFunc("GET /api/dashboard", a.signedIn(a.dashboard))
	mux.HandleFunc("GET /api/profile", a.signedIn(a.profile))
	mux.HandleFunc("PUT /api/profile", a.signedIn(a.editProfile))
	mux.HandleFunc("PUT /api/profile/password", a.signedIn(a.changePassword))
	mux.HandleFunc("GET /api/admin/overview", a.adminOnly(a.adminOverview))
	mux.HandleFunc("GET /api/admin/users", a.adminOnly(a.users))
	mux.HandleFunc("POST /api/admin/users", a.adminOnly(a.createUser))
	mux.HandleFunc("GET /api/admin/users/{id}", a.adminOnly(a.showUser))
	mux.HandleFunc("PUT /api/admin/users/{id}", a.adminOnly(a.updateUser))
	mux.HandleFunc("DELETE /api/admin/users/{id}", a.adminOnly(a.deleteUser))
	for _, m := range a.modules {
		a.mountModule(mux, m)
	}
	for _, route := range []string{"/{$}", "/signin", "/signup", "/app", "/dashboard", "/admin", "/profile"} {
		mux.HandleFunc("GET "+route, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, filepath.Join(a.staticDir, "index.html"))
		})
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(a.staticDir, "assets")))))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			w.Header().Set("Cache-Control", "no-store")
			if r.Method != "GET" && r.Method != "HEAD" && (r.Header.Get("X-Requested-With") != "GoWebStarter" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != a.origin)) {
				problem(w, 403, "This request could not be verified.")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("set DATABASE_URL to a PostgreSQL connection string")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	a, err := newApp(db, envOr("APP_ORIGIN", "http://localhost:8080"), envOr("STATIC_DIR", "web/dist"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err = migrate(ctx, db)
	cancel()
	if err != nil {
		return err
	}
	if len(os.Args) > 1 {
		if len(os.Args) != 4 || os.Args[1] != "admin" || os.Args[2] != "promote" {
			return errors.New("usage: website admin promote <existing email>")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := promote(ctx, db, os.Args[3]); err != nil {
			return err
		}
		log.Print("Administrator access is active. Sign in again to open /admin.")
		return nil
	}
	stop, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	go a.prune(stop)
	server := &http.Server{Addr: envOr("LISTEN_ADDR", ":8080"), Handler: a.routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-stop.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("Serving %s", a.origin)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
