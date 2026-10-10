package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/yaniscalo36-commits/puissance-4/game"
)

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (a *App) handleLobby(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	open, err := a.store.OpenGames(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	active, err := a.store.ActiveGames(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	stats, err := a.store.UserStats(user.ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	leaders, err := a.store.Leaderboard(10)
	if err != nil {
		a.serverError(w, err)
		return
	}

	a.render(w, r, "lobby", map[string]any{
		"Open":    open,
		"Active":  active,
		"Stats":   stats,
		"Leaders": leaders,
	})
}

func (a *App) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	id, err := a.store.CreateGame(currentUser(r).ID)
	if err != nil {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/games/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) handleJoinGame(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := a.store.JoinGame(id, currentUser(r).ID); err != nil && err != ErrCannotJoin {
		a.serverError(w, err)
		return
	}
	// Si la place était déjà prise, la page de la partie l'affichera.
	http.Redirect(w, r, "/games/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (a *App) handleGamePage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	g, err := a.store.GetGame(id)
	if err == ErrGameNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		a.serverError(w, err)
		return
	}
	a.render(w, r, "game", map[string]any{"Game": g})
}

// ---------- API JSON (utilisée par static/game.js) ----------

type gameState struct {
	ID      int64      `json:"id"`
	Board   [][]int    `json:"board"`
	Turn    int        `json:"turn"`
	Status  string     `json:"status"`
	Winner  int        `json:"winner"`
	You     int        `json:"you"` // 0 = spectateur
	P1      string     `json:"p1"`
	P2      string     `json:"p2"`
	LastCol int        `json:"lastCol"`
	Moves   int        `json:"moves"`
	WinLine []game.Pos `json:"winLine"`
}

func stateOf(g *Game, userID int64) gameState {
	_, line := g.Board.WinLine()
	return gameState{
		ID:      g.ID,
		Board:   g.Board.Rows2D(),
		Turn:    int(g.Turn),
		Status:  g.Status,
		Winner:  int(g.Winner),
		You:     int(g.Seat(userID)),
		P1:      g.P1Name,
		P2:      g.P2Name,
		LastCol: g.LastCol,
		Moves:   g.Moves,
		WinLine: line,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (a *App) handleGameState(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		jsonError(w, http.StatusNotFound, ErrGameNotFound.Error())
		return
	}
	g, err := a.store.GetGame(id)
	if err == ErrGameNotFound {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		a.serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stateOf(g, currentUser(r).ID))
}

func (a *App) handleMove(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		jsonError(w, http.StatusNotFound, ErrGameNotFound.Error())
		return
	}
	var body struct {
		Col int `json:"col"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&body); err != nil {
		jsonError(w, http.StatusBadRequest, "requête invalide")
		return
	}

	user := currentUser(r)
	g, err := a.store.PlayMove(id, user.ID, body.Col)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, stateOf(g, user.ID))
	case err == ErrGameNotFound:
		jsonError(w, http.StatusNotFound, err.Error())
	case err == ErrNotPlayer:
		jsonError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, ErrNotYourTurn), errors.Is(err, ErrNotPlaying),
		errors.Is(err, game.ErrColumnFull), errors.Is(err, game.ErrInvalidColumn):
		jsonError(w, http.StatusConflict, err.Error())
	default:
		a.serverError(w, err)
	}
}

func (a *App) handleForfeit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	err := a.store.Forfeit(id, currentUser(r).ID)
	if err != nil && err != ErrNotPlaying && err != ErrNotPlayer && err != ErrGameNotFound {
		a.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
