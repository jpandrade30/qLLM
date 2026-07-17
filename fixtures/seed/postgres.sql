CREATE TABLE IF NOT EXISTS customers (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

TRUNCATE customers;

INSERT INTO customers (id, email, created_at) VALUES
  ('c1', 'alice@example.com', '2024-01-01T00:00:00Z'),
  ('c2', 'bob@example.com', '2024-02-01T00:00:00Z'),
  ('c3', 'carol@example.com', '2024-03-01T00:00:00Z');
