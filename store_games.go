package main

import (
	"database/sql"
	"errors"
	"time"

	"github.com/yaniscalo36-commits/puissance-4/game"
)

const (
	StatusWaiting  = "waiting"
	StatusPlaying  = "playing"
	StatusFinished = "finished"
)

var (
	ErrGameNotFound = errors.New("partie introuvable")
	ErrNotPlayer    = errors.New("vous ne participez pas à cette partie")
	ErrNotYourTurn  = errors.New("ce n'est pas votre tour")
	ErrNotPlaying   = errors.New("la partie n'est pas en cours")
	ErrCannotJoin   = errors.New("impossible de rejoindre cette partie")
)

type Game struct {
	ID        int64
	P1ID      int64
	P1Name    string
	P2ID      int64 // 0 tant que personne n'a rejoint
	P2Name    string
	Board     game.Board
	Turn      game.Cell
	Status    string
	Winner    game.Cell
	LastCol   int
	Moves     int
	UpdatedAt int64
}

// Seat renvoie le numéro de joueur (1 ou 2) de l'utilisateur, ou Empty s'il ne joue pas.
func (g *Game) Seat(userID int64) game.Cell {
	switch userID {
	case g.P1ID:
		return game.Player1
	case g.P2ID:
		return game.Player2
	}
	return game.Empty
}

const gameSelect = `
	SELECT g.id, g.player1_id, u1.username, COALESCE(g.player2_id, 0), COALESCE(u2.username, ''),
	       g.board, g.turn, g.status, g.winner, g.last_col, g.moves, g.updated_at
	FROM games g
	JOIN users u1 ON u1.id = g.player1_id
	LEFT JOIN users u2 ON u2.id = g.player2_id`

type scanner interface{ Scan(...any) error }

func scanGame(row scanner) (*Game, error) {
	var g Game
	var board string
	err := row.Scan(&g.ID, &g.P1ID, &g.P1Name, &g.P2ID, &g.P2Name,
		&board, &g.Turn, &g.Status, &g.Winner, &g.LastCol, &g.Moves, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, err
	}
	if g.Board, err = game.Decode(board); err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *Store) CreateGame(userID int64) (int64, error) {
	var empty game.Board
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`INSERT INTO games (player1_id, board, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		userID, empty.Encode(), now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) GetGame(id int64) (*Game, error) {
	return scanGame(s.db.QueryRow(gameSelect+` WHERE g.id = ?`, id))
}

// JoinGame installe l'utilisateur comme joueur 2 si la place est libre.
func (s *Store) JoinGame(id, userID int64) error {
	res, err := s.db.Exec(
		`UPDATE games SET player2_id = ?, status = ?, updated_at = ?
		 WHERE id = ? AND status = ? AND player1_id != ?`,
		userID, StatusPlaying, time.Now().Unix(), id, StatusWaiting, userID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrCannotJoin
	}
	return nil
}

// PlayMove joue un coup dans une transaction : lecture, vérification des règles, écriture.
func (s *Store) PlayMove(id, userID int64, col int) (*Game, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	g, err := scanGame(tx.QueryRow(gameSelect+` WHERE g.id = ?`, id))
	if err != nil {
		return nil, err
	}
	seat := g.Seat(userID)
	switch {
	case seat == game.Empty:
		return nil, ErrNotPlayer
	case g.Status != StatusPlaying:
		return nil, ErrNotPlaying
	case g.Turn != seat:
		return nil, ErrNotYourTurn
	}

	if _, err := g.Board.Drop(col, seat); err != nil {
		return nil, err
	}
	g.LastCol = col
	g.Moves++
	if winner, _ := g.Board.WinLine(); winner != game.Empty {
		g.Status, g.Winner = StatusFinished, winner
	} else if g.Board.Full() {
		g.Status = StatusFinished
	} else {
		g.Turn = seat.Other()
	}
	g.UpdatedAt = time.Now().Unix()

	_, err = tx.Exec(
		`UPDATE games SET board = ?, turn = ?, status = ?, winner = ?, last_col = ?, moves = ?, updated_at = ?
		 WHERE id = ?`,
		g.Board.Encode(), g.Turn, g.Status, g.Winner, g.LastCol, g.Moves, g.UpdatedAt, g.ID,
	)
	if err != nil {
		return nil, err
	}
	return g, tx.Commit()
}

// Forfeit fait abandonner l'utilisateur : son adversaire gagne.
func (s *Store) Forfeit(id, userID int64) error {
	g, err := s.GetGame(id)
	if err != nil {
		return err
	}
	seat := g.Seat(userID)
	if seat == game.Empty {
		return ErrNotPlayer
	}
	switch g.Status {
	case StatusWaiting:
		// Personne n'a rejoint : on supprime simplement la partie.
		_, err = s.db.Exec(`DELETE FROM games WHERE id = ? AND status = ?`, id, StatusWaiting)
	case StatusPlaying:
		_, err = s.db.Exec(
			`UPDATE games SET status = ?, winner = ?, updated_at = ? WHERE id = ? AND status = ?`,
			StatusFinished, seat.Other(), time.Now().Unix(), id, StatusPlaying,
		)
	default:
		err = ErrNotPlaying
	}
	return err
}

// ---------- Salon ----------

type LobbyGame struct {
	ID       int64
	Host     string
	Opponent string
	Status   string
	MyTurn   bool
}

// OpenGames liste les parties en attente d'un adversaire (hors celles de l'utilisateur).
func (s *Store) OpenGames(userID int64) ([]LobbyGame, error) {
	rows, err := s.db.Query(
		`SELECT g.id, u.username FROM games g JOIN users u ON u.id = g.player1_id
		 WHERE g.status = ? AND g.player1_id != ?
		 ORDER BY g.created_at DESC LIMIT 20`,
		StatusWaiting, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LobbyGame
	for rows.Next() {
		var lg LobbyGame
		if err := rows.Scan(&lg.ID, &lg.Host); err != nil {
			return nil, err
		}
		out = append(out, lg)
	}
	return out, rows.Err()
}

// ActiveGames liste les parties en attente ou en cours de l'utilisateur.
func (s *Store) ActiveGames(userID int64) ([]LobbyGame, error) {
	rows, err := s.db.Query(gameSelect+`
		WHERE g.status IN (?, ?) AND (g.player1_id = ? OR g.player2_id = ?)
		ORDER BY g.updated_at DESC`,
		StatusWaiting, StatusPlaying, userID, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LobbyGame
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		lg := LobbyGame{ID: g.ID, Status: g.Status, MyTurn: g.Status == StatusPlaying && g.Turn == g.Seat(userID)}
		if g.P1ID == userID {
			lg.Opponent = g.P2Name
		} else {
			lg.Opponent = g.P1Name
		}
		out = append(out, lg)
	}
	return out, rows.Err()
}

type Stats struct {
	Wins, Losses, Draws int
}

func (s Stats) Played() int { return s.Wins + s.Losses + s.Draws }

func (s *Store) UserStats(userID int64) (Stats, error) {
	var st Stats
	err := s.db.QueryRow(
		`SELECT
		   COALESCE(SUM(CASE WHEN (player1_id = ?1 AND winner = 1) OR (player2_id = ?1 AND winner = 2) THEN 1 ELSE 0 END), 0),
		   COALESCE(SUM(CASE WHEN (player1_id = ?1 AND winner = 2) OR (player2_id = ?1 AND winner = 1) THEN 1 ELSE 0 END), 0),
		   COALESCE(SUM(CASE WHEN winner = 0 THEN 1 ELSE 0 END), 0)
		 FROM games
		 WHERE status = 'finished' AND (player1_id = ?1 OR player2_id = ?1)`,
		userID,
	).Scan(&st.Wins, &st.Losses, &st.Draws)
	return st, err
}

type LeaderRow struct {
	Username string
	Wins     int
	Played   int
}

func (s *Store) Leaderboard(limit int) ([]LeaderRow, error) {
	rows, err := s.db.Query(
		`SELECT u.username,
		        COALESCE(SUM(CASE WHEN (g.player1_id = u.id AND g.winner = 1) OR (g.player2_id = u.id AND g.winner = 2) THEN 1 ELSE 0 END), 0) AS wins,
		        COUNT(g.id) AS played
		 FROM users u
		 LEFT JOIN games g ON g.status = 'finished' AND (g.player1_id = u.id OR g.player2_id = u.id)
		 GROUP BY u.id
		 HAVING played > 0
		 ORDER BY wins DESC, played ASC, u.username
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderRow
	for rows.Next() {
		var r LeaderRow
		if err := rows.Scan(&r.Username, &r.Wins, &r.Played); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
