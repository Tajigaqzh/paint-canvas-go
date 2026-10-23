package model

import (
	"encoding/json"
	"time"
)

type Document struct {
	ID        string          `gorm:"primaryKey;size:36" json:"id"`
	// Nullable for legacy SQLite rows created before account ownership existed.
	UserID    *string         `gorm:"index;size:36" json:"-"`
	Name      string          `gorm:"not null;size:160" json:"name"`
	Data      json.RawMessage `gorm:"type:text;not null" json:"data"`
	Revision  uint64          `gorm:"not null;default:1" json:"revision"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type User struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null;size:254" json:"email"`
	DisplayName  string    `gorm:"not null;size:80" json:"displayName"`
	PasswordHash string    `gorm:"not null" json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
