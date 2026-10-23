package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"paint-canvas-go/internal/model"
)

func Open(dsn string) (*gorm.DB, error) {
	if strings.HasPrefix(dsn, "file:") {
		path := strings.TrimPrefix(strings.SplitN(dsn, "?", 2)[0], "file:")
		if path != ":memory:" && path != "" {
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				return nil, err
			}
		}
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&model.User{}, &model.Document{}); err != nil {
		return nil, err
	}
	return db, nil
}
