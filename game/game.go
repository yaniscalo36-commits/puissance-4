// Package game contient les règles du Puissance 4, indépendamment du web.
package game

import (
	"errors"
	"strings"
)

const (
	Rows = 6
	Cols = 7
)

// Cell représente le contenu d'une case : vide, joueur 1 ou joueur 2.
type Cell int

const (
	Empty Cell = iota
	Player1
	Player2
)

// Other renvoie l'adversaire du joueur.
func (c Cell) Other() Cell {
	if c == Player1 {
		return Player2
	}
	return Player1
}

// Board est la grille de jeu. La ligne 0 est en haut, la ligne Rows-1 en bas.
type Board [Rows][Cols]Cell

// Pos est une position (ligne, colonne) sur la grille.
type Pos struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

var (
	ErrInvalidColumn = errors.New("colonne invalide")
	ErrColumnFull    = errors.New("cette colonne est pleine")
)

// Drop fait tomber un jeton du joueur p dans la colonne col
// et renvoie la ligne où il s'est arrêté.
func (b *Board) Drop(col int, p Cell) (int, error) {
	if col < 0 || col >= Cols {
		return -1, ErrInvalidColumn
	}
	for row := Rows - 1; row >= 0; row-- {
		if b[row][col] == Empty {
			b[row][col] = p
			return row, nil
		}
	}
	return -1, ErrColumnFull
}

// Full indique si la grille est entièrement remplie (match nul).
func (b *Board) Full() bool {
	for col := 0; col < Cols; col++ {
		if b[0][col] == Empty {
			return false
		}
	}
	return true
}

// directions à vérifier : horizontale, verticale et les deux diagonales.
var directions = [4][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}

// WinLine renvoie les 4 cases alignées du gagnant, ou nil s'il n'y en a pas.
func (b *Board) WinLine() (Cell, []Pos) {
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			p := b[row][col]
			if p == Empty {
				continue
			}
			for _, d := range directions {
				line := []Pos{{row, col}}
				r, c := row+d[0], col+d[1]
				for len(line) < 4 && inside(r, c) && b[r][c] == p {
					line = append(line, Pos{r, c})
					r, c = r+d[0], c+d[1]
				}
				if len(line) == 4 {
					return p, line
				}
			}
		}
	}
	return Empty, nil
}

func inside(r, c int) bool {
	return r >= 0 && r < Rows && c >= 0 && c < Cols
}

// Encode transforme la grille en une chaîne de 42 caractères ('0', '1', '2')
// pour la stocker en base de données.
func (b *Board) Encode() string {
	var sb strings.Builder
	sb.Grow(Rows * Cols)
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			sb.WriteByte(byte('0' + b[row][col]))
		}
	}
	return sb.String()
}

// Decode reconstruit une grille à partir de Encode.
func Decode(s string) (Board, error) {
	var b Board
	if len(s) != Rows*Cols {
		return b, errors.New("grille corrompue : taille invalide")
	}
	for i := 0; i < len(s); i++ {
		v := s[i] - '0'
		if v > 2 {
			return b, errors.New("grille corrompue : caractère invalide")
		}
		b[i/Cols][i%Cols] = Cell(v)
	}
	return b, nil
}

// Rows2D renvoie la grille sous forme de tableau d'entiers (pratique pour le JSON).
func (b *Board) Rows2D() [][]int {
	out := make([][]int, Rows)
	for row := range out {
		out[row] = make([]int, Cols)
		for col := range out[row] {
			out[row][col] = int(b[row][col])
		}
	}
	return out
}
