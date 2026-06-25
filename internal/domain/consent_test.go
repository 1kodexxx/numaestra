package domain

import "testing"

func TestValidateConsentDocVersion(t *testing.T) {
	if err := ValidateConsentDocVersion(CurrentConsentDocVersion); err != nil {
		t.Fatalf("ожидали успех: %v", err)
	}
	if err := ValidateConsentDocVersion(""); err != ErrConsentRequired {
		t.Fatalf("пустая версия: %v", err)
	}
	if err := ValidateConsentDocVersion("old"); err != ErrInvalidConsentVersion {
		t.Fatalf("устаревшая версия: %v", err)
	}
}
