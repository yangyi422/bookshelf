package book

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bookshelf/internal/database"
	"bookshelf/internal/metadata"
	"bookshelf/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("book not found")
var ErrDuplicate = errors.New("file already exists")

type Service struct {
	db        *gorm.DB
	storage   *storage.Storage
	maxUpload int64
}

func NewService(db *gorm.DB, store *storage.Storage, maxUpload int64) *Service {
	return &Service{db: db, storage: store, maxUpload: maxUpload}
}

type Input struct {
	Title         string   `json:"title"`
	Subtitle      string   `json:"subtitle"`
	Description   string   `json:"description"`
	Language      string   `json:"language"`
	Publisher     string   `json:"publisher"`
	ISBN          string   `json:"isbn"`
	ReadingStatus string   `json:"reading_status"`
	Rating        int      `json:"rating"`
	AuthorIDs     []string `json:"author_ids"`
	TagIDs        []string `json:"tag_ids"`
}
type Detail struct {
	database.Book
	Files   []database.BookFile `json:"files"`
	Authors []database.Author   `json:"authors"`
	Tags    []database.Tag      `json:"tags"`
}
type ListOptions struct {
	Keyword, AuthorID, TagID, Format, ReadingStatus, Sort, Order string
	Page, Size                                                   int
}

func (s *Service) Create(in Input) (Detail, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return Detail{}, errors.New("title is required")
	}
	if in.ReadingStatus == "" {
		in.ReadingStatus = "unread"
	}
	if err := validateInput(in); err != nil {
		return Detail{}, err
	}
	b := database.Book{ID: uuid.NewString(), Title: in.Title, Subtitle: in.Subtitle, Description: in.Description, Language: in.Language, Publisher: in.Publisher, ISBN: in.ISBN, ReadingStatus: in.ReadingStatus, Rating: in.Rating}
	dir := s.storage.Relative("books", b.ID)
	if _, err := s.storage.EnsureDir(s.storage.Relative(dir, "files")); err != nil {
		return Detail{}, err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
		return replaceLinks(tx, b.ID, in.AuthorIDs, in.TagIDs)
	}); err != nil {
		_ = s.storage.Remove(dir)
		return Detail{}, err
	}
	if err := s.writeMetadata(b); err != nil {
		_ = s.db.Delete(&b)
		_ = s.storage.Remove(dir)
		return Detail{}, err
	}
	return s.Get(b.ID, false)
}
func validateInput(in Input) error {
	statuses := map[string]bool{"unread": true, "reading": true, "finished": true, "paused": true, "abandoned": true}
	if !statuses[in.ReadingStatus] {
		return errors.New("invalid reading_status")
	}
	if in.Rating < 0 || in.Rating > 5 {
		return errors.New("rating must be between 0 and 5")
	}
	return nil
}
func replaceLinks(tx *gorm.DB, id string, authors, tags []string) error {
	if err := tx.Where("book_id = ?", id).Delete(&database.BookAuthor{}).Error; err != nil {
		return err
	}
	if err := tx.Where("book_id = ?", id).Delete(&database.BookTag{}).Error; err != nil {
		return err
	}
	for i, aid := range authors {
		var n int64
		if err := tx.Model(&database.Author{}).Where("id = ?", aid).Count(&n).Error; err != nil || n != 1 {
			return fmt.Errorf("author %q does not exist", aid)
		}
		if err := tx.Create(&database.BookAuthor{BookID: id, AuthorID: aid, Position: i}).Error; err != nil {
			return err
		}
	}
	for _, tid := range tags {
		var n int64
		if err := tx.Model(&database.Tag{}).Where("id = ?", tid).Count(&n).Error; err != nil || n != 1 {
			return fmt.Errorf("tag %q does not exist", tid)
		}
		if err := tx.Create(&database.BookTag{BookID: id, TagID: tid}).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Get(id string, includeDeleted bool) (Detail, error) {
	var d Detail
	q := s.db.Where("id = ?", id)
	if !includeDeleted {
		q = q.Where("deleted_at IS NULL")
	}
	if err := q.First(&d.Book).Error; err != nil {
		return d, ErrNotFound
	}
	s.db.Where("book_id = ?", id).Order("created_at").Find(&d.Files)
	s.db.Raw("SELECT authors.* FROM authors JOIN book_authors ON authors.id=book_authors.author_id WHERE book_authors.book_id=? ORDER BY book_authors.position", id).Scan(&d.Authors)
	s.db.Raw("SELECT tags.* FROM tags JOIN book_tags ON tags.id=book_tags.tag_id WHERE book_tags.book_id=? ORDER BY tags.name", id).Scan(&d.Tags)
	return d, nil
}
func (s *Service) List(o ListOptions) ([]Detail, int64, error) {
	if o.Page < 1 {
		o.Page = 1
	}
	if o.Size < 1 || o.Size > 100 {
		o.Size = 20
	}
	q := s.db.Model(&database.Book{}).Where("books.deleted_at IS NULL")
	if o.Keyword != "" {
		like := "%" + o.Keyword + "%"
		q = q.Where("books.title LIKE ? OR books.subtitle LIKE ? OR books.isbn LIKE ? OR EXISTS (SELECT 1 FROM book_authors ba JOIN authors a ON a.id=ba.author_id WHERE ba.book_id=books.id AND a.name LIKE ?) OR EXISTS (SELECT 1 FROM book_tags bt JOIN tags t ON t.id=bt.tag_id WHERE bt.book_id=books.id AND t.name LIKE ?)", like, like, like, like, like)
	}
	if o.AuthorID != "" {
		q = q.Where("EXISTS (SELECT 1 FROM book_authors WHERE book_authors.book_id=books.id AND author_id=?)", o.AuthorID)
	}
	if o.TagID != "" {
		q = q.Where("EXISTS (SELECT 1 FROM book_tags WHERE book_tags.book_id=books.id AND tag_id=?)", o.TagID)
	}
	if o.Format != "" {
		q = q.Where("EXISTS (SELECT 1 FROM book_files WHERE book_files.book_id=books.id AND format=?)", strings.ToLower(o.Format))
	}
	if o.ReadingStatus != "" {
		q = q.Where("books.reading_status=?", o.ReadingStatus)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []database.Book
	sorts := map[string]string{"created_at": "books.created_at", "updated_at": "books.updated_at", "title": "books.title", "rating": "books.rating"}
	sortCol := sorts[o.Sort]
	if sortCol == "" {
		sortCol = "books.created_at"
	}
	order := "DESC"
	if strings.EqualFold(o.Order, "asc") {
		order = "ASC"
	}
	if err := q.Order(sortCol + " " + order).Offset((o.Page - 1) * o.Size).Limit(o.Size).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]Detail, 0, len(rows))
	for _, b := range rows {
		d, err := s.Get(b.ID, false)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, d)
	}
	return out, total, nil
}
func (s *Service) Trash() ([]Detail, error) {
	var rows []database.Book
	if err := s.db.Where("deleted_at IS NOT NULL").Order("deleted_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Detail, 0, len(rows))
	for _, b := range rows {
		d, err := s.Get(b.ID, true)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}
func (s *Service) Update(id string, in Input) (Detail, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return Detail{}, errors.New("title is required")
	}
	if err := validateInput(in); err != nil {
		return Detail{}, err
	}
	var b database.Book
	if err := s.db.Where("id=? AND deleted_at IS NULL", id).First(&b).Error; err != nil {
		return Detail{}, ErrNotFound
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"title": in.Title, "subtitle": in.Subtitle, "description": in.Description, "language": in.Language, "publisher": in.Publisher, "isbn": in.ISBN, "reading_status": in.ReadingStatus, "rating": in.Rating}
		if err := tx.Model(&b).Updates(updates).Error; err != nil {
			return err
		}
		return replaceLinks(tx, id, in.AuthorIDs, in.TagIDs)
	})
	if err != nil {
		return Detail{}, err
	}
	d, err := s.Get(id, false)
	if err == nil {
		err = s.writeMetadata(d.Book)
	}
	return d, err
}
func (s *Service) Delete(id string) error {
	var b database.Book
	if err := s.db.Where("id=? AND deleted_at IS NULL", id).First(&b).Error; err != nil {
		return ErrNotFound
	}
	src := s.storage.Relative("books", id)
	dst := s.storage.Relative("trash", id+"-"+time.Now().UTC().Format("20060102T150405.000000000"))
	if err := s.storage.Move(src, dst); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := s.db.Model(&b).Updates(map[string]any{"deleted_at": &now, "trash_path": dst}).Error; err != nil {
		_ = s.storage.Move(dst, src)
		return err
	}
	return nil
}
func (s *Service) Restore(id string) error {
	var b database.Book
	if err := s.db.Where("id=? AND deleted_at IS NOT NULL", id).First(&b).Error; err != nil {
		return ErrNotFound
	}
	if b.TrashPath == "" {
		return errors.New("book has no recoverable trash path")
	}
	dst := s.storage.Relative("books", id)
	if err := s.storage.Move(b.TrashPath, dst); err != nil {
		return err
	}
	if err := s.db.Model(&b).Updates(map[string]any{"deleted_at": nil, "trash_path": ""}).Error; err != nil {
		_ = s.storage.Move(dst, b.TrashPath)
		return err
	}
	return nil
}
func (s *Service) AddFile(bookID string, fh *multipart.FileHeader) (database.BookFile, error) {
	var result database.BookFile
	if fh.Size > s.maxUpload {
		return result, errors.New("file exceeds configured upload limit")
	}
	if _, err := s.Get(bookID, false); err != nil {
		return result, err
	}
	format, mime, err := storage.FormatForName(fh.Filename)
	if err != nil {
		return result, err
	}
	jobID := uuid.NewString()
	tempRel := s.storage.Relative("imports", jobID+filepath.Ext(strings.ToLower(fh.Filename)))
	tempPath, err := s.storage.Resolve(tempRel)
	if err != nil {
		return result, err
	}
	src, err := fh.Open()
	if err != nil {
		return result, err
	}
	defer src.Close()
	out, err := os.OpenFile(tempPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, err
	}
	h := sha256.New()
	n, copyErr := io.Copy(io.MultiWriter(out, h), io.LimitReader(src, s.maxUpload+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || n > s.maxUpload {
		_ = s.storage.Remove(tempRel)
		if n > s.maxUpload {
			return result, errors.New("file exceeds configured upload limit")
		}
		if copyErr != nil {
			return result, copyErr
		}
		return result, closeErr
	}
	headerFile, _ := os.Open(tempPath)
	header := make([]byte, 512)
	hn, _ := headerFile.Read(header)
	_ = headerFile.Close()
	if err := storage.ValidateHeader(format, header[:hn]); err != nil {
		_ = s.storage.Remove(tempRel)
		return result, err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	var count int64
	if err := s.db.Model(&database.BookFile{}).Where("sha256=?", sum).Count(&count).Error; err != nil {
		_ = s.storage.Remove(tempRel)
		return result, err
	}
	if count > 0 {
		_ = s.storage.Remove(tempRel)
		return result, ErrDuplicate
	}
	fileID := uuid.NewString()
	finalRel := s.storage.Relative("books", bookID, "files", fileID+"."+format)
	result = database.BookFile{ID: fileID, BookID: bookID, Format: format, MIMEType: mime, RelativePath: finalRel, OriginalName: filepath.Base(fh.Filename), FileSize: n, SHA256: sum}
	tx := s.db.Begin()
	if tx.Error != nil {
		_ = s.storage.Remove(tempRel)
		return result, tx.Error
	}
	if err := tx.Create(&result).Error; err != nil {
		tx.Rollback()
		_ = s.storage.Remove(tempRel)
		return result, err
	}
	if err := s.storage.Move(tempRel, finalRel); err != nil {
		tx.Rollback()
		_ = s.storage.Remove(tempRel)
		return result, err
	}
	if err := tx.Commit().Error; err != nil {
		_ = s.storage.Remove(finalRel)
		return result, err
	}
	d, _ := s.Get(bookID, false)
	_ = s.writeMetadata(d.Book)
	return result, nil
}
func (s *Service) File(bookID, fileID string) (database.BookFile, *os.File, error) {
	var f database.BookFile
	if err := s.db.Raw("SELECT book_files.* FROM book_files JOIN books ON books.id=book_files.book_id WHERE book_files.id=? AND book_files.book_id=? AND books.deleted_at IS NULL", fileID, bookID).Scan(&f).Error; err != nil || f.ID == "" {
		return f, nil, ErrNotFound
	}
	handle, err := s.storage.Open(f.RelativePath)
	return f, handle, err
}
func (s *Service) DeleteFile(bookID, fileID string) error {
	var f database.BookFile
	if err := s.db.Where("id=? AND book_id=?", fileID, bookID).First(&f).Error; err != nil {
		return ErrNotFound
	}
	trash := s.storage.Relative("trash", "files", fileID+"."+f.Format)
	if err := s.storage.Move(f.RelativePath, trash); err != nil {
		return err
	}
	if err := s.db.Delete(&f).Error; err != nil {
		_ = s.storage.Move(trash, f.RelativePath)
		return err
	}
	return nil
}
func (s *Service) Import(fh *multipart.FileHeader) (Detail, error) {
	title := strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))
	d, err := s.Create(Input{Title: title, ReadingStatus: "unread"})
	if err != nil {
		return d, err
	}
	f, err := s.AddFile(d.ID, fh)
	if err != nil {
		s.purgeImport(d.ID)
		return Detail{}, err
	}
	full, err := s.storage.Resolve(f.RelativePath)
	if err != nil {
		s.purgeImport(d.ID)
		return Detail{}, err
	}
	parsed, err := metadata.Parse(full, f.Format)
	if err != nil {
		s.purgeImport(d.ID)
		return Detail{}, err
	}
	authorIDs := []string{}
	for _, name := range parsed.Authors {
		var a database.Author
		if e := s.db.Where("name=?", name).First(&a).Error; e != nil {
			a = database.Author{ID: uuid.NewString(), Name: name, SortName: name}
			if e = s.db.Create(&a).Error; e != nil {
				s.purgeImport(d.ID)
				return Detail{}, e
			}
		}
		authorIDs = append(authorIDs, a.ID)
	}
	parsedTitle := parsed.Title
	if parsedTitle == "" {
		parsedTitle = d.Title
	}
	in := Input{Title: parsedTitle, Description: parsed.Description, Language: parsed.Language, Publisher: parsed.Publisher, ISBN: metadata.ISBN(parsed.Identifier), ReadingStatus: "unread", AuthorIDs: authorIDs}
	if _, err = s.Update(d.ID, in); err != nil {
		s.purgeImport(d.ID)
		return Detail{}, err
	}
	if parsed.PageCount > 0 {
		if err = s.db.Model(&database.BookFile{}).Where("id=?", f.ID).Update("page_count", parsed.PageCount).Error; err != nil {
			s.purgeImport(d.ID)
			return Detail{}, err
		}
	}
	if len(parsed.Cover) > 0 {
		if err = s.saveCover(d.ID, parsed.Cover, parsed.CoverMIME, parsed.CoverExt); err != nil {
			s.purgeImport(d.ID)
			return Detail{}, err
		}
	}
	return s.Get(d.ID, false)
}
func (s *Service) purgeImport(id string) {
	_ = s.db.Where("book_id=?", id).Delete(&database.BookFile{}).Error
	_ = s.db.Where("book_id=?", id).Delete(&database.BookAuthor{}).Error
	_ = s.db.Where("book_id=?", id).Delete(&database.BookTag{}).Error
	_ = s.db.Delete(&database.Book{}, "id=?", id).Error
	_ = s.storage.Remove(s.storage.Relative("books", id))
}
func (s *Service) saveCover(bookID string, data []byte, mime, ext string) error {
	_ = mime
	rel := s.storage.Relative("books", bookID, "cover"+ext)
	full, err := s.storage.Resolve(rel)
	if err != nil {
		return err
	}
	if err = os.WriteFile(full, data, 0640); err != nil {
		return err
	}
	if err = s.db.Model(&database.Book{}).Where("id=?", bookID).Update("cover_path", rel).Error; err != nil {
		_ = os.Remove(full)
		return err
	}
	return nil
}
func (s *Service) SetCover(bookID string, fh *multipart.FileHeader) error {
	if _, err := s.Get(bookID, false); err != nil {
		return err
	}
	if fh.Size > 10<<20 {
		return errors.New("cover exceeds 10 MB")
	}
	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	data, err := io.ReadAll(io.LimitReader(src, (10<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 10<<20 {
		return errors.New("cover exceeds 10 MB")
	}
	ext, mime := metadata.DetectCover(data, fh.Header.Get("Content-Type"))
	if ext == "" {
		return errors.New("cover must be JPEG, PNG, or WebP")
	}
	return s.saveCover(bookID, data, mime, ext)
}
func (s *Service) Cover(bookID string) (string, *os.File, error) {
	var b database.Book
	if err := s.db.Where("id=? AND deleted_at IS NULL", bookID).First(&b).Error; err != nil || b.CoverPath == "" {
		return "", nil, ErrNotFound
	}
	f, err := s.storage.Open(b.CoverPath)
	if err != nil {
		return "", nil, err
	}
	mime := "image/jpeg"
	if strings.HasSuffix(b.CoverPath, ".png") {
		mime = "image/png"
	} else if strings.HasSuffix(b.CoverPath, ".webp") {
		mime = "image/webp"
	}
	return mime, f, nil
}
func (s *Service) writeMetadata(b database.Book) error {
	d, err := s.Get(b.ID, true)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	path, err := s.storage.Resolve(s.storage.Relative("books", b.ID, "metadata.json"))
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0640)
}
