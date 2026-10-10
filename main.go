package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed templates static
var assets embed.FS

type App struct {
	store   *Store
	pages   map[string]*template.Template
	limiter *loginLimiter
}

var pageNames = []string{"login", "register", "home"}

func loadPages() map[string]*template.Template {
	pages := map[string]*template.Template{}
	for _, name := range pageNames {
		pages[name] = template.Must(template.ParseFS(assets, "templates/layout.html", "templates/"+name+".html"))
	}
	return pages
}

func (a *App) render(w http.ResponseWriter, r *http.Request, page string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	data["User"] = currentUser(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.pages[page].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("template %s : %v", page, err)
	}
}

func isAPI(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	store, err := OpenStore(getenv("DB_PATH", "puissance4.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	app := &App{
		store:   store,
		pages:   loadPages(),
		limiter: newLoginLimiter(5, 5*time.Minute),
	}

	// Nettoyage régulier des sessions expirées.
	go func() {
		for {
			store.DeleteExpiredSessions()
			time.Sleep(time.Hour)
		}
	}()

	staticFS, _ := fs.Sub(assets, "static")

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	mux.HandleFunc("GET /register", app.handleRegisterPage)
	mux.HandleFunc("POST /register", app.handleRegister)
	mux.HandleFunc("GET /login", app.handleLoginPage)
	mux.HandleFunc("POST /login", app.handleLogin)
	mux.HandleFunc("POST /logout", app.handleLogout)

	mux.HandleFunc("GET /{$}", requireUser(func(w http.ResponseWriter, r *http.Request) {
		app.render(w, r, "home", nil)
	}))

	handler := securityHeaders(sameOrigin(app.withUser(mux)))

	addr := ":" + getenv("PORT", "8080")
	log.Printf("Puissance 4 démarré sur http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
