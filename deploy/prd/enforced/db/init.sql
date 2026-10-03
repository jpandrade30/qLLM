CREATE TABLE orders (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL,
  item TEXT NOT NULL,
  total_cents INTEGER NOT NULL
);

CREATE TABLE products (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  price_cents INTEGER NOT NULL
);

INSERT INTO orders (id, user_id, item, total_cents) VALUES
  ('o-42-1', '42', 'Notebook', 5300),
  ('o-42-2', '42', 'Pen', 200),
  ('o-7-1', '7', 'Headphones', 8900);

INSERT INTO products (id, name, price_cents) VALUES
  ('p-1', 'Notebook', 5300),
  ('p-2', 'Pen', 200),
  ('p-3', 'Headphones', 8900);
