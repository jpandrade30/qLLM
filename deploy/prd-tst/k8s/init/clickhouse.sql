CREATE TABLE IF NOT EXISTS default.gps_samples
(
  vehicle_id String,
  ts DateTime,
  speed_kph Float64
)
ENGINE = MergeTree
ORDER BY (vehicle_id, ts);
INSERT INTO default.gps_samples VALUES
  ('veh-1', '2026-09-29 12:00:00', 42),
  ('veh-2', '2026-09-29 12:01:00', 0);
