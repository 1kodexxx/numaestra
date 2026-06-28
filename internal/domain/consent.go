package domain

import "errors"

// CurrentConsentDocVersion — версия пакета юридических документов (оферта + согласие).
// При обновлении текстов в /legal/* увеличьте версию — старые клиенты получат 400.
const CurrentConsentDocVersion = "2026-06-28"

var (
	ErrConsentRequired       = errors.New("необходимо согласие с условиями и обработкой персональных данных")
	ErrInvalidConsentVersion = errors.New("устаревшая версия согласия, обновите страницу")
)

func ValidateConsentDocVersion(version string) error {
	if version == "" {
		return ErrConsentRequired
	}
	if version != CurrentConsentDocVersion {
		return ErrInvalidConsentVersion
	}
	return nil
}
