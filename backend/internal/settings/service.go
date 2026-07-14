package settings

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/url"
	"strings"
	"time"

	"bookshelf/internal/auth"
	"bookshelf/internal/database"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	ModeDisabled     = "disabled"
	ModeHTTPSOnly    = "https_only"
	ModeHTTPAndHTTPS = "http_and_https"
)

type Defaults struct {
	Enabled       bool
	AccessMode    string
	Username      string
	Password      string
	PublicBaseURL string
}

type View struct {
	OPDSEnabled        bool   `json:"opds_enabled"`
	OPDSAccessMode     string `json:"opds_access_mode"`
	OPDSUsername       string `json:"opds_username"`
	PasswordConfigured bool   `json:"opds_password_configured"`
	PublicBaseURL      string `json:"public_base_url"`
	OPDSURL            string `json:"opds_url"`
}

type Update struct {
	OPDSEnabled         bool   `json:"opds_enabled"`
	OPDSAccessMode      string `json:"opds_access_mode"`
	OPDSUsername        string `json:"opds_username"`
	OPDSPassword        string `json:"opds_password"`
	PublicBaseURL       string `json:"public_base_url"`
	ConfirmInsecureHTTP bool   `json:"confirm_insecure_http"`
}

type SetupInput struct {
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
	Update
}

type Service struct {
	db       *gorm.DB
	defaults Defaults
}

func New(db *gorm.DB, defaults Defaults) *Service { return &Service{db: db, defaults: defaults} }

func (s *Service) Initialized() (bool, error) {
	var count int64
	err := s.db.Model(&database.User{}).Count(&count).Error
	return count > 0, err
}

func (s *Service) EnsureExistingInstall() error {
	initialized, err := s.Initialized()
	if err != nil || !initialized {
		return err
	}
	var count int64
	if err := s.db.Model(&database.SystemSetting{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	mode := normalizeMode(s.defaults.AccessMode, s.defaults.Enabled)
	hash := ""
	enabled := s.defaults.Enabled
	if enabled && (strings.TrimSpace(s.defaults.Username) == "" || s.defaults.Password == "") {
		enabled = false
		mode = ModeDisabled
	} else if s.defaults.Password != "" {
		b, err := auth.HashPassword(s.defaults.Password)
		if err != nil {
			return err
		}
		hash = b
	}
	row := database.SystemSetting{ID: 1, OPDSEnabled: enabled, OPDSAccessMode: mode, OPDSUsername: strings.TrimSpace(s.defaults.Username), OPDSPasswordHash: hash, PublicBaseURL: strings.TrimRight(strings.TrimSpace(s.defaults.PublicBaseURL), "/")}
	return s.db.Create(&row).Error
}

func (s *Service) Current() (database.SystemSetting, error) {
	var row database.SystemSetting
	err := s.db.First(&row, 1).Error
	return row, err
}

func (s *Service) View() (View, error) {
	row, err := s.Current()
	if err != nil {
		return View{}, err
	}
	return view(row), nil
}

func (s *Service) Initialize(in SetupInput) (View, error) {
	initialized, err := s.Initialized()
	if err != nil {
		return View{}, err
	}
	if initialized {
		return View{}, errors.New("initialization has already completed")
	}
	if strings.TrimSpace(in.AdminUsername) == "" || len(in.AdminPassword) < 12 {
		return View{}, errors.New("administrator username and a password of at least 12 characters are required")
	}
	if in.OPDSEnabled && in.OPDSPassword == in.AdminPassword {
		return View{}, errors.New("OPDS password must be different from the administrator password")
	}
	setting, err := validatedSetting(in.Update, "")
	if err != nil {
		return View{}, err
	}
	adminHash, err := auth.HashPassword(in.AdminPassword)
	if err != nil {
		return View{}, err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		u := database.User{ID: uuid.NewString(), Username: strings.TrimSpace(in.AdminUsername), PasswordHash: adminHash, DisplayName: strings.TrimSpace(in.AdminUsername), Role: "admin", Enabled: true}
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		setting.ID = 1
		return tx.Create(&setting).Error
	})
	return view(setting), err
}

func (s *Service) Update(in Update, admin database.User) (View, error) {
	current, err := s.Current()
	if err != nil {
		return View{}, err
	}
	if in.OPDSPassword != "" && bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(in.OPDSPassword)) == nil {
		return View{}, errors.New("OPDS password must be different from the administrator password")
	}
	setting, err := validatedSetting(in, current.OPDSPasswordHash)
	if err != nil {
		return View{}, err
	}
	setting.ID = current.ID
	setting.CreatedAt = current.CreatedAt
	setting.UpdatedAt = time.Now().UTC()
	if err := s.db.Save(&setting).Error; err != nil {
		return View{}, err
	}
	return view(setting), nil
}

func (s *Service) VerifyCredentials(username, password string) bool {
	row, err := s.Current()
	return err == nil && row.OPDSEnabled && subtleUser(username, row.OPDSUsername) && bcrypt.CompareHashAndPassword([]byte(row.OPDSPasswordHash), []byte(password)) == nil
}

func validatedSetting(in Update, existingHash string) (database.SystemSetting, error) {
	mode := normalizeMode(in.OPDSAccessMode, in.OPDSEnabled)
	if in.OPDSEnabled && mode == ModeDisabled {
		return database.SystemSetting{}, errors.New("enabled OPDS requires an access mode")
	}
	if mode == ModeHTTPAndHTTPS && !in.ConfirmInsecureHTTP {
		return database.SystemSetting{}, errors.New("HTTP security warning must be confirmed")
	}
	username := strings.TrimSpace(in.OPDSUsername)
	if in.OPDSEnabled && username == "" {
		return database.SystemSetting{}, errors.New("OPDS username is required")
	}
	hash := existingHash
	if in.OPDSPassword != "" {
		var err error
		hash, err = auth.HashPassword(in.OPDSPassword)
		if err != nil {
			return database.SystemSetting{}, err
		}
	}
	if in.OPDSEnabled && hash == "" {
		return database.SystemSetting{}, errors.New("a new OPDS password is required")
	}
	base := strings.TrimRight(strings.TrimSpace(in.PublicBaseURL), "/")
	if base != "" {
		u, err := url.ParseRequestURI(base)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
			return database.SystemSetting{}, errors.New("public_base_url must be an absolute HTTP(S) origin")
		}
	}
	return database.SystemSetting{OPDSEnabled: in.OPDSEnabled, OPDSAccessMode: mode, OPDSUsername: username, OPDSPasswordHash: hash, PublicBaseURL: base}, nil
}

func normalizeMode(mode string, enabled bool) string {
	if !enabled {
		return ModeDisabled
	}
	if mode == ModeHTTPSOnly || mode == ModeHTTPAndHTTPS {
		return mode
	}
	return ModeHTTPSOnly
}

func view(row database.SystemSetting) View {
	u := ""
	if row.PublicBaseURL != "" && row.OPDSEnabled {
		u = strings.TrimRight(row.PublicBaseURL, "/") + "/opds"
	}
	return View{OPDSEnabled: row.OPDSEnabled, OPDSAccessMode: row.OPDSAccessMode, OPDSUsername: row.OPDSUsername, PasswordConfigured: row.OPDSPasswordHash != "", PublicBaseURL: row.PublicBaseURL, OPDSURL: u}
}

func subtleUser(a, b string) bool {
	ah := sha256.Sum256([]byte(a))
	bh := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ah[:], bh[:]) == 1
}
