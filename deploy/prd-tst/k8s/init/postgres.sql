CREATE TABLE depots (
  id TEXT PRIMARY KEY,
  code TEXT NOT NULL,
  city TEXT NOT NULL
);
CREATE TABLE vehicles (
  id TEXT PRIMARY KEY,
  vin TEXT NOT NULL,
  depot_id TEXT NOT NULL REFERENCES depots (id),
  status TEXT NOT NULL
);
INSERT INTO depots (id, code, city) VALUES
  ('dep-n', 'YARD-N', 'Natal'),
  ('dep-s', 'YARD-S', 'Recife');
INSERT INTO vehicles (id, vin, depot_id, status) VALUES
  ('veh-1', '9BWZZZ377VT004251', 'dep-n', 'active'),
  ('veh-2', '9BWZZZ377VT004252', 'dep-s', 'maintenance');
