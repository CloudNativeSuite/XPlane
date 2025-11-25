package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

type Server struct {
	DB *sql.DB
}

type NodeRegisterRequest struct {
	ServiceDomain string  `json:"service_domain"`
	IP            string  `json:"ip"`
	Port          *int    `json:"port,omitempty"`
	Region        *string `json:"region,omitempty"`
	Role          *string `json:"role,omitempty"`
	BaseWeight    *int    `json:"base_weight,omitempty"`
}

type NodeDeregisterRequest struct {
	ServiceDomain string `json:"service_domain"`
	IP            string `json:"ip"`
}

type NodeHeartbeatRequest struct {
	ServiceDomain string `json:"service_domain"`
	IP            string `json:"ip"`
}

func (s *Server) RegisterNode(w http.ResponseWriter, r *http.Request) {
	var req NodeRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Role == nil {
		def := "active"
		req.Role = &def
	}
	if req.BaseWeight == nil {
		def := 100
		req.BaseWeight = &def
	}

	tx, err := s.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var svcID int64
	err = tx.QueryRow("SELECT id FROM services WHERE domain=$1", req.ServiceDomain).Scan(&svcID)
	if err == sql.ErrNoRows {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var nodeID int64
	err = tx.QueryRow("SELECT id FROM nodes WHERE service_id=$1 AND ip=$2", svcID, req.IP).Scan(&nodeID)

	now := time.Now().UTC()

	if err == sql.ErrNoRows {
		res, err := tx.Exec(`
INSERT INTO nodes (service_id, ip, port, region, role, base_weight, status, last_seen_at)
VALUES ($1,$2,$3,$4,$5,$6,'unknown',$7)
`, svcID, req.IP, req.Port, req.Region, *req.Role, *req.BaseWeight, now)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		nodeID, _ = res.LastInsertId()
	} else if err == nil {
		_, err = tx.Exec(`
UPDATE nodes
SET port=$1, region=$2, role=$3, base_weight=$4, status='unknown', last_seen_at=$5
WHERE id=$6
`, req.Port, req.Region, *req.Role, *req.BaseWeight, now, nodeID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"node_id": nodeID,
	})
}

func (s *Server) DeregisterNode(w http.ResponseWriter, r *http.Request) {
	var req NodeDeregisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tx, err := s.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	var svcID int64
	err = tx.QueryRow("SELECT id FROM services WHERE domain=$1", req.ServiceDomain).Scan(&svcID)
	if err == sql.ErrNoRows {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "msg": "service not found"})
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err := tx.Exec("DELETE FROM nodes WHERE service_id=$1 AND ip=$2", svcID, req.IP); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req NodeHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()

	if _, err := s.DB.Exec(`
UPDATE nodes
SET last_seen_at=$1
WHERE service_id = (SELECT id FROM services WHERE domain=$2)
AND ip = $3
`, now, req.ServiceDomain, req.IP); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *Server) CleanupStale(w http.ResponseWriter, r *http.Request) {
	threshold := time.Now().UTC().Add(-10 * time.Minute)
	if _, err := s.DB.Exec("UPDATE nodes SET status='down' WHERE last_seen_at < $1", threshold); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
