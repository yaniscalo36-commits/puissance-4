// Gère l'affichage de la grille et les coups joués.
// L'état de la partie est récupéré toutes les secondes auprès du serveur (polling).

const ROWS = 6;
const COLS = 7;
const POLL_MS = 1000;

const root = document.getElementById("game");
const gameId = root.dataset.id;
const boardEl = document.getElementById("board");
const statusEl = document.getElementById("status");

const els = {
  p1: document.getElementById("player1"),
  p2: document.getElementById("player2"),
  p1Name: document.getElementById("p1-name"),
  p2Name: document.getElementById("p2-name"),
  share: document.getElementById("share"),
  shareLink: document.getElementById("share-link"),
  copy: document.getElementById("copy-link"),
  join: document.getElementById("join-form"),
  newGame: document.getElementById("new-form"),
  forfeit: document.getElementById("forfeit-form"),
};

let state = null;
let sending = false;
let pollTimer = null;

// ---------- Construction de la grille ----------

const columns = [];
const cells = [];

for (let c = 0; c < COLS; c++) {
  const col = document.createElement("button");
  col.type = "button";
  col.className = "column";
  col.setAttribute("aria-label", `Colonne ${c + 1}`);
  col.addEventListener("click", () => play(c));
  cells[c] = [];
  for (let r = 0; r < ROWS; r++) {
    const cell = document.createElement("span");
    cell.className = "cell";
    col.appendChild(cell);
    cells[c][r] = cell;
  }
  columns.push(col);
  boardEl.appendChild(col);
}

// Jouer au clavier : touches 1 à 7.
document.addEventListener("keydown", e => {
  const n = parseInt(e.key, 10);
  if (n >= 1 && n <= COLS && !e.target.matches("input")) play(n - 1);
});

// ---------- Rendu ----------

function render(next) {
  const prev = state;
  state = next;
  const myTurn = next.status === "playing" && next.you === next.turn;

  // Grille
  const win = new Set((next.winLine || []).map(p => `${p.row}-${p.col}`));
  for (let c = 0; c < COLS; c++) {
    for (let r = 0; r < ROWS; r++) {
      const cell = cells[c][r];
      const v = next.board[r][c];
      cell.className = "cell" + (v ? ` p${v}` : "") + (win.has(`${r}-${c}`) ? " win" : "");
    }
    const full = next.board[0][c] !== 0;
    columns[c].disabled = !myTurn || full;
  }
  boardEl.classList.toggle("my-turn", myTurn);
  boardEl.dataset.you = next.you;

  // Animation du dernier jeton tombé
  if (next.lastCol >= 0 && (!prev || prev.moves !== next.moves)) {
    const r = next.board.findIndex(row => row[next.lastCol] !== 0);
    if (r >= 0) {
      const cell = cells[next.lastCol][r];
      cell.style.setProperty("--fall", r + 1);
      cell.classList.add("drop");
    }
  }

  // Joueurs
  els.p1Name.textContent = next.p1 + (next.you === 1 ? " (vous)" : "");
  els.p2Name.textContent = next.p2 ? next.p2 + (next.you === 2 ? " (vous)" : "") : "…";
  els.p1.classList.toggle("active", next.status === "playing" && next.turn === 1);
  els.p2.classList.toggle("active", next.status === "playing" && next.turn === 2);

  // Message
  statusEl.className = "status";
  if (next.status === "waiting") {
    statusEl.textContent = next.you ? "En attente d'un adversaire…" : `${next.p1} attend un adversaire.`;
  } else if (next.status === "playing") {
    const name = next.turn === 1 ? next.p1 : next.p2;
    if (myTurn) {
      statusEl.textContent = "À vous de jouer !";
      statusEl.classList.add("turn");
    } else {
      statusEl.textContent = `Au tour de ${name}…`;
    }
  } else if (next.winner === 0) {
    statusEl.textContent = "Match nul !";
  } else if (next.you === next.winner) {
    statusEl.textContent = "Victoire ! 🎉";
    statusEl.classList.add("won");
  } else if (next.you) {
    statusEl.textContent = "Défaite… La revanche ?";
    statusEl.classList.add("lost");
  } else {
    statusEl.textContent = `${next.winner === 1 ? next.p1 : next.p2} a gagné !`;
  }

  // Actions
  els.share.hidden = !(next.status === "waiting" && next.you === 1);
  els.join.hidden = !(next.status === "waiting" && next.you === 0);
  els.forfeit.hidden = !(next.you && next.status !== "finished");
  els.forfeit.querySelector("button").textContent = next.status === "waiting" ? "Annuler la partie" : "Abandonner";
  els.newGame.hidden = next.status !== "finished";

  if (next.status === "finished") stopPolling();
}

// ---------- Réseau ----------

async function fetchState() {
  try {
    const res = await fetch(`/api/games/${gameId}`, { headers: { Accept: "application/json" } });
    if (res.status === 401) return location.assign("/login");
    if (res.status === 404) {
      stopPolling();
      statusEl.textContent = "Cette partie n'existe plus.";
      return;
    }
    const data = await res.json();
    if (!state || data.moves !== state.moves || data.status !== state.status || data.p2 !== state.p2) {
      render(data);
    }
  } catch {
    // Réseau indisponible : on réessaiera au prochain tour.
  }
}

async function play(col) {
  if (sending || !state || state.status !== "playing" || state.you !== state.turn) return;
  sending = true;
  try {
    const res = await fetch(`/api/games/${gameId}/move`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ col }),
    });
    const data = await res.json();
    if (res.ok) {
      render(data);
    } else {
      flash(data.error || "Coup refusé");
    }
  } catch {
    flash("Connexion perdue, réessayez.");
  } finally {
    sending = false;
  }
}

function flash(msg) {
  statusEl.textContent = msg;
  statusEl.className = "status lost";
}

function startPolling() {
  fetchState();
  pollTimer = setInterval(fetchState, POLL_MS);
}

function stopPolling() {
  clearInterval(pollTimer);
  pollTimer = null;
}

// ---------- Partage du lien ----------

els.shareLink.value = location.href;
els.copy.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(location.href);
    els.copy.textContent = "Copié ✓";
  } catch {
    els.shareLink.select();
  }
});

els.forfeit.addEventListener("submit", e => {
  const msg = state && state.status === "waiting" ? "Annuler cette partie ?" : "Abandonner ? Votre adversaire gagnera la partie.";
  if (!confirm(msg)) e.preventDefault();
});

startPolling();
