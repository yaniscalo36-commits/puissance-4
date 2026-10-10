package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store regroupe tous les accès à la base SQLite.
type Store struct {
	db *sql.DB
}

type User struct {
	ID       int64
	Username string
}

var ErrUsernameTaken = errors.New("ce pseudo est déjà pris")

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
		password_hash TEXT    NOT NULL,
		created_at    INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS sessions (
		token      TEXT    PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at INTEGER NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS games (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		player1_id INTEGER NOT NULL REFERENCES users(id),
		player2_id INTEGER REFERENCES users(id),
		board      TEXT    NOT NULL,
		turn       INTEGER NOT NULL DEFAULT 1,
		status     TEXT    NOT NULL DEFAULT 'waiting',
		winner     INTEGER NOT NULL DEFAULT 0,
		last_col   INTEGER NOT NULL DEFAULT -1,
		moves      INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_games_status ON games(status)`,
}

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// SQLite n'accepte qu'un seul écrivain à la fois : une seule connexion
	// évite les erreurs "database is locked" et sérialise les coups joués.
	db.SetMaxOpenConns(1)

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return nil, fmt.Errorf("migration : %w", err)
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ---------- Utilisateurs ----------

func (s *Store) CreateUser(username, passwordHash string) (*User, error) {
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, time.Now().Unix(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, ErrUsernameTaken
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username}, nil
}

// UserCredentials renvoie l'utilisateur et le hash de son mot de passe.
func (s *Store) UserCredentials(username string) (*User, string, error) {
	var u User
	var hash string
	err := s.db.QueryRow(
		`SELECT id, username, password_hash FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &hash)
	if err != nil {
		return nil, "", err
	}
	return &u, hash, nil
}

// ---------- Sessions ----------

func (s *Store) CreateSession(token string, userID int64, expires time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expires.Unix(),
	)
	return err
}

func (s *Store) UserBySession(token string) (*User, error) {
	var u User
	err := s.db.QueryRow(
		`SELECT u.id, u.username FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token = ? AND s.expires_at > ?`,
		token, time.Now().Unix(),
	).Scan(&u.ID, &u.Username)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, time.Now().Unix())
	return err
}
