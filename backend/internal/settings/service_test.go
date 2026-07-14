package settings

import (
	"encoding/json"
	"strings"
	"testing"

	"bookshelf/internal/auth"
	"bookshelf/internal/database"
)

func testService(t *testing.T) (*Service, database.User) {
	t.Helper()
	db, err := database.Open(t.TempDir(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := auth.HashPassword("administrator-password")
	admin := database.User{ID: "admin", Username: "admin", DisplayName: "admin", Role: "admin", Enabled: true, PasswordHash: hash}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatal(err)
	}
	return New(db, Defaults{}), admin
}

func TestInitializeDefaultsAndPasswordRules(t *testing.T) {
	db, err := database.Open(t.TempDir(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(db, Defaults{})
	_, err = svc.Initialize(SetupInput{AdminUsername: "admin", AdminPassword: "same-password-value", Update: Update{OPDSEnabled: true, OPDSAccessMode: ModeHTTPSOnly, OPDSUsername: "reader", OPDSPassword: "same-password-value"}})
	if err == nil {
		t.Fatal("same administrator and OPDS password accepted")
	}
	view, err := svc.Initialize(SetupInput{AdminUsername: "admin", AdminPassword: "administrator-password", Update: Update{OPDSEnabled: true, OPDSAccessMode: "", OPDSUsername: "reader", OPDSPassword: "independent-reader-password"}})
	if err != nil || view.OPDSAccessMode != ModeHTTPSOnly || !view.PasswordConfigured {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	b, _ := json.Marshal(view)
	if strings.Contains(string(b), "independent-reader-password") || strings.Contains(string(b), "password_hash") {
		t.Fatal("password leaked")
	}
}

func TestUpdateRequiresWarningAndAppliesImmediately(t *testing.T) {
	svc, admin := testService(t)
	hash, _ := auth.HashPassword("reader-password-value")
	if err := svc.db.Create(&database.SystemSetting{ID: 1, OPDSEnabled: true, OPDSAccessMode: ModeHTTPSOnly, OPDSUsername: "reader", OPDSPasswordHash: hash}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := svc.Update(Update{OPDSEnabled: true, OPDSAccessMode: ModeHTTPAndHTTPS, OPDSUsername: "reader"}, admin)
	if err == nil {
		t.Fatal("missing warning confirmation accepted")
	}
	view, err := svc.Update(Update{OPDSEnabled: true, OPDSAccessMode: ModeHTTPAndHTTPS, OPDSUsername: "reader", ConfirmInsecureHTTP: true}, admin)
	if err != nil || view.OPDSAccessMode != ModeHTTPAndHTTPS {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	_, err = svc.Update(Update{OPDSEnabled: true, OPDSAccessMode: ModeHTTPSOnly, OPDSUsername: "reader", OPDSPassword: "administrator-password"}, admin)
	if err == nil {
		t.Fatal("administrator password accepted for OPDS")
	}
}
