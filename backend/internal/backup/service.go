package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bookshelf/internal/database"
	"bookshelf/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db        *gorm.DB
	store     *storage.Storage
	retention time.Duration
}

func New(db *gorm.DB, store *storage.Storage, days int) *Service {
	return &Service{db: db, store: store, retention: time.Duration(days) * 24 * time.Hour}
}
func (s *Service) Create() (database.BackupRecord, error) {
	id := uuid.NewString()
	cacheRel := s.store.Relative("cache", "backup-"+id)
	cache, err := s.store.EnsureDir(cacheRel)
	if err != nil {
		return database.BackupRecord{}, err
	}
	defer s.store.Remove(cacheRel)
	snapshot := filepath.Join(cache, "library.db")
	if err = s.db.Exec("VACUUM INTO ?", snapshot).Error; err != nil {
		return database.BackupRecord{}, fmt.Errorf("create SQLite snapshot: %w", err)
	}
	name := "bookshelf-" + time.Now().UTC().Format("20060102-150405") + ".tar.gz"
	rel := s.store.Relative("backups", name)
	target, err := s.store.Resolve(rel)
	if err != nil {
		return database.BackupRecord{}, err
	}
	if err = writeArchive(target, s.store.Root(), snapshot); err != nil {
		_ = os.Remove(target)
		return database.BackupRecord{}, err
	}
	f, err := os.Open(target)
	if err != nil {
		return database.BackupRecord{}, err
	}
	sum, err := storage.SHA256(f)
	_ = f.Close()
	if err != nil {
		return database.BackupRecord{}, err
	}
	if err = os.WriteFile(target+".sha256", []byte(sum+"  "+filepath.Base(target)+"\n"), 0600); err != nil {
		_ = os.Remove(target)
		return database.BackupRecord{}, err
	}
	st, err := os.Stat(target)
	if err != nil {
		return database.BackupRecord{}, err
	}
	record := database.BackupRecord{ID: id, FilePath: rel, FileSize: st.Size(), Checksum: sum, CreatedAt: time.Now().UTC()}
	if err = s.db.Create(&record).Error; err != nil {
		_ = os.Remove(target)
		_ = os.Remove(target + ".sha256")
		return record, err
	}
	_ = s.prune()
	return record, nil
}
func writeArchive(target, root, snapshot string) error {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	closeAll := func() error {
		if e := tw.Close(); e != nil {
			return e
		}
		if e := gz.Close(); e != nil {
			return e
		}
		return out.Close()
	}
	if err = addFile(tw, snapshot, "library.db"); err != nil {
		_ = closeAll()
		return err
	}
	for _, dir := range []string{"books", "manifests"} {
		base := filepath.Join(root, dir)
		err = filepath.Walk(base, func(p string, info os.FileInfo, e error) error {
			if e != nil {
				return e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			rel, e := filepath.Rel(root, p)
			if e != nil {
				return e
			}
			name := filepath.ToSlash(rel)
			if info.IsDir() {
				return nil
			}
			return addFile(tw, p, name)
		})
		if err != nil {
			_ = closeAll()
			return err
		}
	}
	cfg, _ := json.MarshalIndent(map[string]any{"format_version": 1, "created_at": time.Now().UTC(), "contents": []string{"library.db", "books", "manifests"}}, "", "  ")
	hdr := &tar.Header{Name: "backup.json", Mode: 0600, Size: int64(len(cfg)), ModTime: time.Now().UTC()}
	if err = tw.WriteHeader(hdr); err == nil {
		_, err = tw.Write(cfg)
	}
	if err != nil {
		_ = closeAll()
		return err
	}
	return closeAll()
}
func addFile(tw *tar.Writer, path, name string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	hdr := &tar.Header{Name: name, Mode: 0600, Size: st.Size(), ModTime: st.ModTime()}
	if err = tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}
func (s *Service) List() ([]database.BackupRecord, error) {
	var rows []database.BackupRecord
	err := s.db.Order("created_at DESC").Find(&rows).Error
	return rows, err
}
func (s *Service) Validate(id string) (database.BackupRecord, error) {
	var r database.BackupRecord
	if err := s.db.First(&r, "id=?", id).Error; err != nil {
		return r, err
	}
	f, err := s.store.Open(r.FilePath)
	if err != nil {
		return r, err
	}
	sum, err := storage.SHA256(f)
	_ = f.Close()
	if err != nil {
		return r, err
	}
	if !strings.EqualFold(sum, r.Checksum) {
		return r, fmt.Errorf("backup checksum mismatch")
	}
	return r, nil
}
func (s *Service) prune() error {
	cutoff := time.Now().Add(-s.retention)
	var old []database.BackupRecord
	if err := s.db.Where("created_at < ?", cutoff).Find(&old).Error; err != nil {
		return err
	}
	for _, r := range old {
		_ = s.store.Remove(r.FilePath)
		_ = s.store.Remove(r.FilePath + ".sha256")
		_ = s.db.Delete(&r).Error
	}
	return nil
}
