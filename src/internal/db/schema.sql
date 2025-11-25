CREATE TABLE IF NOT EXISTS services (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    domain TEXT NOT NULL UNIQUE,
    dns_provider TEXT NOT NULL,
    dns_config JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS nodes (
    id SERIAL PRIMARY KEY,
    service_id INTEGER NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    ip TEXT NOT NULL,
    port INTEGER,
    region TEXT,
    role TEXT DEFAULT 'active',
    base_weight INTEGER DEFAULT 100,
    status TEXT DEFAULT 'unknown',
    latency_ms REAL,
    cpu_load REAL,
    last_seen_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS health_logs (
    id SERIAL PRIMARY KEY,
    node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    success BOOLEAN,
    latency_ms REAL,
    cpu_load REAL,
    raw_blackbox TEXT,
    raw_node TEXT,
    raw_process TEXT
);
