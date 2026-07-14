package database

import (
	"testing"
)

func TestOpenMigratesCoreTables(t *testing.T) {
	db, err := Open(t.TempDir(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []any{&User{}, &Session{}, &SystemSetting{}, &Book{}, &BookFile{}, &Author{}, &Tag{}, &ImportJob{}, &BackupRecord{}} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("missing table for %T", model)
		}
	}
	var mode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal mode = %q", mode)
	}
	var versions int64
	if err := db.Model(&schemaMigration{}).Count(&versions).Error; err != nil || versions != 4 {
		t.Fatalf("migration versions = %d: %v", versions, err)
	}
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
}
