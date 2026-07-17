CREATE DATABASE IF NOT EXISTS billing;
USE billing;

CREATE TABLE IF NOT EXISTS invoices (
  id VARCHAR(64) PRIMARY KEY,
  customer_id VARCHAR(64) NOT NULL,
  total_cents INT NOT NULL,
  status VARCHAR(32) NOT NULL
);

TRUNCATE invoices;

INSERT INTO invoices (id, customer_id, total_cents, status) VALUES
  ('i1', 'c1', 19950, 'paid'),
  ('i2', 'c1', 5000, 'paid'),
  ('i3', 'c2', 12000, 'open'),
  ('i4', 'c3', 8000, 'paid');
