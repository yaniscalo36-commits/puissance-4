package main

import (
	"log"
	"net/http"
	"strings"
)

func (a *App) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, r, "register", nil)
}

func (a *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")

	fail := func(msg string) {
		w.WriteHeader(http.StatusBadRequest)
		a.render(w, r, "register", map[string]any{"Error": msg, "Username": username})
	}

	switch {
	case !usernameRe.MatchString(username):
		fail("Le pseudo doit faire 3 à 20 caractères (lettres, chiffres, _ ou -).")
		return
	case len(password) < minPasswordLen:
		fail("Le mot de passe doit faire au moins 8 caractères.")
		return
	case len(password) > 72:
		fail("Le mot de passe est trop long (72 caractères maximum).")
		return
	case password != confirm:
		fail("Les deux mots de passe ne correspondent pas.")
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		a.serverError(w, err)
		return
	}
	user, err := a.store.CreateUser(username, hash)
	if err == ErrUsernameTaken {
		fail("Ce pseudo est déjà pris.")
		return
	}
	if err != nil {
		a.serverError(w, err)
		return
	}
	if err := a.startSession(w, r, user.ID); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if currentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	a.render(w, r, "login", nil)
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	fail := func(status int, msg string) {
		w.WriteHeader(status)
		a.render(w, r, "login", map[string]any{"Error": msg, "Username": username})
	}

	if a.limiter.Blocked(ip) {
		fail(http.StatusTooManyRequests, "Trop de tentatives. Réessayez dans quelques minutes.")
		return
	}

	user, hash, err := a.store.UserCredentials(username)
	if err != nil {
		hash = string(dummyHash)
	}
	if !checkPassword(hash, password) || user == nil {
		a.limiter.Fail(ip)
		// Même message que le pseudo existe ou non.
		fail(http.StatusUnauthorized, "Pseudo ou mot de passe incorrect.")
		return
	}

	a.limiter.Reset(ip)
	if err := a.startSession(w, r, user.ID); err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.endSession(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) serverError(w http.ResponseWriter, err error) {
	log.Printf("erreur serveur : %v", err)
	http.Error(w, "Erreur interne du serveur", http.StatusInternalServerError)
}
