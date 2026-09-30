import sqlite3
import os

path = os.environ.get("QLLM_FLEET_SQLITE_PATH", "/data/fleet.db")
os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
con = sqlite3.connect(path)
con.execute(
    "CREATE TABLE IF NOT EXISTS shift_notes (id TEXT PRIMARY KEY, vehicle_id TEXT NOT NULL, body TEXT NOT NULL)"
)
con.execute("DELETE FROM shift_notes")
con.execute(
    "INSERT INTO shift_notes (id, vehicle_id, body) VALUES ('n1', 'veh-1', 'pre-trip ok')"
)
con.commit()
con.close()
print("sqlite seed ok", path)
