package combine

const traceSchemaSQL = `
CREATE TABLE IF NOT EXISTS merge_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  job_id TEXT UNIQUE,
  role TEXT NOT NULL,
  status TEXT NOT NULL,
  requested_interval_unit INTEGER,
  final_quota INTEGER,
  final_name TEXT,
  final_group TEXT,
  error TEXT,
  rollback_attempted INTEGER NOT NULL DEFAULT 0,
  rollback_succeeded INTEGER NOT NULL DEFAULT 0,
  rollback_note TEXT,
  delete_started INTEGER NOT NULL DEFAULT 0,
  old_cards_deleted_count INTEGER NOT NULL DEFAULT 0,
  created_card_id INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  completed_at INTEGER
);
CREATE TABLE IF NOT EXISTS merge_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  merge_id INTEGER NOT NULL REFERENCES merge_records(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK (kind IN ('source', 'result')),
  token_id INTEGER,
  key_full TEXT NOT NULL,
  key_hash TEXT NOT NULL,
  key_mask TEXT NOT NULL,
  name TEXT,
  remain_quota INTEGER,
  used_quota INTEGER,
  interval_unit INTEGER,
  group_name TEXT,
  status INTEGER,
  delete_ok INTEGER,
  delete_error TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE (merge_id, kind, key_hash)
);
CREATE INDEX IF NOT EXISTS idx_merge_tokens_key_hash ON merge_tokens(key_hash);
CREATE INDEX IF NOT EXISTS idx_merge_tokens_merge_id ON merge_tokens(merge_id);
CREATE INDEX IF NOT EXISTS idx_merge_records_job_id ON merge_records(job_id);
CREATE TABLE IF NOT EXISTS generated_tokens (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  token_id INTEGER NOT NULL,
  key_full TEXT NOT NULL,
  key_hash TEXT NOT NULL UNIQUE,
  key_mask TEXT NOT NULL,
  name TEXT,
  remain_quota INTEGER,
  used_quota INTEGER,
  interval_unit INTEGER,
  group_name TEXT,
  status INTEGER,
  raw_json TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_generated_tokens_key_hash ON generated_tokens(key_hash);
`
