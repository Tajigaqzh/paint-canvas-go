package store

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOpenMigratesLegacyDocumentsTable(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	legacyDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.Exec(`CREATE TABLE documents (
		id text PRIMARY KEY,
		name text NOT NULL,
		data text NOT NULL,
		revision integer NOT NULL DEFAULT 1,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.Exec(`INSERT INTO documents (id, name, data, revision) VALUES (?, ?, ?, ?)`,
		"legacy-id", "Legacy", `{"pages":{}}`, 1).Error; err != nil {
		t.Fatal(err)
	}

	db, err := Open(dsn)
	if err != nil {
		t.Fatalf("open should migrate a legacy documents table: %v", err)
	}
	var document struct {
		ID     string
		UserID *string `gorm:"column:user_id"`
	}
	if err := db.Table("documents").Select("id, user_id").First(&document, "id = ?", "legacy-id").Error; err != nil {
		t.Fatal(err)
	}
	if document.UserID != nil {
		t.Fatalf("legacy document owner = %v, want nil", *document.UserID)
	}
}
