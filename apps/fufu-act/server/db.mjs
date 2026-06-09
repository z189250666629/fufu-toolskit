import Database from 'better-sqlite3';
import { mkdirSync } from 'fs';
import { dirname } from 'path';

export function createDb(dbPath) {
  mkdirSync(dirname(dbPath), { recursive: true });
  const db = new Database(dbPath);

  db.pragma('journal_mode = WAL');

  db.exec(`
    CREATE TABLE IF NOT EXISTS cards (
      card_key TEXT PRIMARY KEY,
      card_name TEXT NOT NULL,
      dollars INTEGER NOT NULL,
      total_spins INTEGER NOT NULL,
      used_spins INTEGER NOT NULL DEFAULT 0,
      won_jackpot INTEGER NOT NULL DEFAULT 0,
      total_won INTEGER NOT NULL DEFAULT 0,
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      last_spin_at TEXT,
      source TEXT NOT NULL DEFAULT 'act',
      purchase_time TEXT
    );

    CREATE TABLE IF NOT EXISTS spin_log (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      card_key TEXT NOT NULL,
      prize_dollars INTEGER NOT NULL,
      is_retry INTEGER NOT NULL DEFAULT 0,
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      FOREIGN KEY (card_key) REFERENCES cards(card_key)
    );

    CREATE TABLE IF NOT EXISTS scratch_games (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      card_key TEXT NOT NULL UNIQUE,
      mine_pos INTEGER NOT NULL,
      revealed TEXT NOT NULL DEFAULT '[]',
      prize_dollars INTEGER NOT NULL DEFAULT 0,
      status TEXT NOT NULL DEFAULT 'playing',
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      FOREIGN KEY (card_key) REFERENCES cards(card_key)
    );

    CREATE TABLE IF NOT EXISTS credit_queue (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      card_key TEXT NOT NULL,
      prize_dollars INTEGER NOT NULL,
      status TEXT NOT NULL DEFAULT 'pending',
      retries INTEGER NOT NULL DEFAULT 0,
      error TEXT,
      created_at TEXT NOT NULL DEFAULT (datetime('now')),
      processed_at TEXT,
      FOREIGN KEY (card_key) REFERENCES cards(card_key)
    );
  `);

  // 迁移：给已有表加新列
  const cols = db.pragma('table_info(cards)').map(c => c.name);
  if (!cols.includes('source')) {
    db.exec("ALTER TABLE cards ADD COLUMN source TEXT NOT NULL DEFAULT 'act'");
  }
  if (!cols.includes('purchase_time')) {
    db.exec("ALTER TABLE cards ADD COLUMN purchase_time TEXT");
  }
  if (!cols.includes('rigged')) {
    db.exec("ALTER TABLE cards ADD COLUMN rigged TEXT");
  }

  return db;
}
