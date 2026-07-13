package database

import (
	"fmt"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type schemaMigration struct {
	Version   uint `gorm:"primaryKey"`
	AppliedAt time.Time
}

func Open(dataDir string, busyTimeoutMS int) (*gorm.DB, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=%d&_foreign_keys=on", filepath.Join(dataDir, "library.db"), busyTimeoutMS)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return nil, fmt.Errorf("enable sqlite WAL: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("run database migrations: %w", err)
	}
	return db, nil
}

func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return err
	}
	var count int64
	if err := db.Model(&schemaMigration{}).Where("version = ?", 1).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&User{}, &Session{}, &Book{}, &BookFile{}, &Author{}, &BookAuthor{}, &Tag{}, &BookTag{}, &ImportJob{}, &BackupRecord{}, &AuditLog{}); err != nil {
				return err
			}
			return tx.Create(&schemaMigration{Version: 1, AppliedAt: time.Now().UTC()}).Error
		}); err != nil {
			return err
		}
	}
	var v2 int64
	if err := db.Model(&schemaMigration{}).Where("version = ?", 2).Count(&v2).Error; err != nil {
		return err
	}
	if v2 == 0 {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Book{}); err != nil {
				return err
			}
			return tx.Create(&schemaMigration{Version: 2, AppliedAt: time.Now().UTC()}).Error
		}); err != nil {
			return err
		}
	}
	var v3 int64
	if err := db.Model(&schemaMigration{}).Where("version = ?", 3).Count(&v3).Error; err != nil {
		return err
	}
	if v3 == 0 {
		return db.Transaction(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&BookFile{}); err != nil {
				return err
			}
			return tx.Create(&schemaMigration{Version: 3, AppliedAt: time.Now().UTC()}).Error
		})
	}
	return nil
}
