package game

import "testing"

func play(t *testing.T, b *Board, p Cell, cols ...int) {
	t.Helper()
	for _, c := range cols {
		if _, err := b.Drop(c, p); err != nil {
			t.Fatalf("Drop(%d): %v", c, err)
		}
	}
}

func TestDropStacksFromBottom(t *testing.T) {
	var b Board
	row, _ := b.Drop(3, Player1)
	if row != Rows-1 {
		t.Fatalf("premier jeton en ligne %d, attendu %d", row, Rows-1)
	}
	row, _ = b.Drop(3, Player2)
	if row != Rows-2 {
		t.Fatalf("deuxième jeton en ligne %d, attendu %d", row, Rows-2)
	}
}

func TestDropErrors(t *testing.T) {
	var b Board
	if _, err := b.Drop(-1, Player1); err != ErrInvalidColumn {
		t.Errorf("colonne -1 : %v", err)
	}
	if _, err := b.Drop(Cols, Player1); err != ErrInvalidColumn {
		t.Errorf("colonne %d : %v", Cols, err)
	}
	for i := 0; i < Rows; i++ {
		b.Drop(0, Player1)
	}
	if _, err := b.Drop(0, Player1); err != ErrColumnFull {
		t.Errorf("colonne pleine : %v", err)
	}
}

func TestWinHorizontal(t *testing.T) {
	var b Board
	play(t, &b, Player1, 0, 1, 2)
	if w, _ := b.WinLine(); w != Empty {
		t.Fatal("victoire détectée trop tôt")
	}
	play(t, &b, Player1, 3)
	w, line := b.WinLine()
	if w != Player1 || len(line) != 4 {
		t.Fatalf("victoire horizontale non détectée : %v %v", w, line)
	}
}

func TestWinVertical(t *testing.T) {
	var b Board
	play(t, &b, Player2, 6, 6, 6, 6)
	if w, _ := b.WinLine(); w != Player2 {
		t.Fatal("victoire verticale non détectée")
	}
}

func TestWinDiagonals(t *testing.T) {
	// Diagonale montante vers la droite (/)
	var b Board
	play(t, &b, Player2, 1, 2, 2, 3, 3, 3)
	play(t, &b, Player1, 0, 1, 2, 3)
	if w, _ := b.WinLine(); w != Player1 {
		t.Fatal("diagonale / non détectée")
	}

	// Diagonale descendante vers la droite (\)
	var b2 Board
	play(t, &b2, Player2, 5, 4, 4, 3, 3, 3)
	play(t, &b2, Player1, 6, 5, 4, 3)
	if w, _ := b2.WinLine(); w != Player1 {
		t.Fatal("diagonale \\ non détectée")
	}
}

func TestFull(t *testing.T) {
	var b Board
	for col := 0; col < Cols; col++ {
		for row := 0; row < Rows; row++ {
			b.Drop(col, Player1)
		}
	}
	if !b.Full() {
		t.Fatal("grille pleine non détectée")
	}
}

func TestEncodeDecode(t *testing.T) {
	var b Board
	play(t, &b, Player1, 0, 3)
	play(t, &b, Player2, 3, 6)
	got, err := Decode(b.Encode())
	if err != nil {
		t.Fatal(err)
	}
	if got != b {
		t.Fatal("Decode(Encode(b)) != b")
	}
	if _, err := Decode("abc"); err == nil {
		t.Fatal("une grille invalide aurait dû être refusée")
	}
}
