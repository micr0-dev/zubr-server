package models

import "time"

type Role string

const (
	RoleOwner Role = "owner"
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type User struct {
	ID           string      `json:"id"`
	Username     string      `json:"username"`
	PasswordHash string      `json:"password_hash"`
	Email        string      `json:"email,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	Active       bool        `json:"active"`
	Banned       bool        `json:"banned"`
	Pending      bool        `json:"pending"` // True if awaiting approval (approval mode)
	Role         Role        `json:"role"`
	Config       *UserConfig `json:"config,omitempty"`
}
