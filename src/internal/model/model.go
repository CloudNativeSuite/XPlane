package model

import "time"

type Service struct {
	ID          int64
	Name        string
	Domain      string
	DNSProvider string
	DNSConfig   []byte
	CreatedAt   time.Time
}

type Node struct {
	ID         int64
	ServiceID  int64
	IP         string
	Port       *int
	Region     *string
	Role       string
	BaseWeight int
	Status     string
	LatencyMs  *float64
	CPULoad    *float64
	LastSeenAt *time.Time
	CreatedAt  time.Time
}
