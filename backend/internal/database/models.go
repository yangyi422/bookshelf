package database

import "time"

type User struct {
	ID           string    `gorm:"type:text;primaryKey" json:"id"`
	Username     string    `gorm:"type:text;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	DisplayName  string    `gorm:"not null" json:"display_name"`
	Role         string    `gorm:"not null" json:"role"`
	Enabled      bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type Session struct {
	IDHash    string    `gorm:"primaryKey"`
	UserID    string    `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
}
type Book struct {
	ID            string     `gorm:"primaryKey" json:"id"`
	Title         string     `gorm:"not null" json:"title"`
	Subtitle      string     `json:"subtitle"`
	Description   string     `json:"description"`
	Language      string     `json:"language"`
	Publisher     string     `json:"publisher"`
	ISBN          string     `json:"isbn"`
	CoverPath     string     `json:"cover_path"`
	ReadingStatus string     `json:"reading_status"`
	PublishedAt   *time.Time `json:"published_at"`
	Rating        int        `json:"rating"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	TrashPath     string     `json:"-"`
}
type BookFile struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	BookID       string    `gorm:"index;not null" json:"book_id"`
	Format       string    `json:"format"`
	MIMEType     string    `json:"mime_type"`
	RelativePath string    `json:"-"`
	OriginalName string    `json:"original_name"`
	FileSize     int64     `json:"file_size"`
	PageCount    int       `json:"page_count,omitempty"`
	SHA256       string    `gorm:"uniqueIndex;not null" json:"sha256"`
	CreatedAt    time.Time `json:"created_at"`
}
type Author struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	SortName  string    `json:"sort_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type BookAuthor struct {
	BookID   string `gorm:"primaryKey"`
	AuthorID string `gorm:"primaryKey"`
	Position int
}
type Tag struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
type BookTag struct {
	BookID string `gorm:"primaryKey"`
	TagID  string `gorm:"primaryKey"`
}
type ImportJob struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	OriginalName string    `json:"original_name"`
	TempPath     string    `json:"-"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type BackupRecord struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	FilePath  string    `json:"file_path"`
	FileSize  int64     `json:"file_size"`
	Checksum  string    `json:"checksum"`
	CreatedAt time.Time `json:"created_at"`
}
type AuditLog struct {
	ID                                                 string `gorm:"primaryKey"`
	UserID, Action, ResourceType, ResourceID, RemoteIP string
	CreatedAt                                          time.Time
}
