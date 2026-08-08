package postgres

import (
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

func scanOrderSnapshot(row pgx.Row) (domain.OrderSnapshot, error) {
	var snap domain.OrderSnapshot
	var demoClipsJSON []byte
	err := row.Scan(
		&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
		&snap.CategoryID, &snap.SunoPrompt,
		&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
		&snap.GenerationPhase, &snap.GenerationProgress, &snap.TracksReady,
		&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken,
		&snap.AdminFeedback, &snap.AdminFeedbackAt,
		&snap.ConsentGivenAt, &snap.ConsentDocVersion,
		&snap.ShareRevokedAt,
		&snap.PromoCodeID, &snap.OriginalAmountKopecks, &snap.DiscountKopecks, &snap.ReferralCode,
		&snap.CreatedAt, &snap.UpdatedAt, &snap.PaidAt, &snap.CompletedAt,
		&snap.DemoStatus, &snap.DemoURL, &snap.DemoAccountID,
		&demoClipsJSON,
		&snap.DemoInvoiceID, &snap.DemoAmountKopecks, &snap.DemoPaymentStatus,
	)
	if err != nil {
		return domain.OrderSnapshot{}, fmt.Errorf("scan order: %w", err)
	}
	if err := unmarshalDemoClips(demoClipsJSON, &snap); err != nil {
		return domain.OrderSnapshot{}, err
	}
	return snap, nil
}

func scanOrderRows(rows pgx.Rows) ([]domain.OrderSnapshot, error) {
	var snaps []domain.OrderSnapshot
	for rows.Next() {
		var snap domain.OrderSnapshot
		var demoClipsJSON []byte
		if err := rows.Scan(
			&snap.ID, &snap.InvoiceID, &snap.CustomerEmail, &snap.CustomerPhone, &snap.Brief,
			&snap.CategoryID, &snap.SunoPrompt,
			&snap.AmountKopecks, &snap.Currency, &snap.PaymentStatus, &snap.GenerationStatus,
			&snap.GenerationPhase, &snap.GenerationProgress, &snap.TracksReady,
			&snap.AssignedAccountID, &snap.FailureReason, &snap.AccessToken,
			&snap.AdminFeedback, &snap.AdminFeedbackAt,
			&snap.ConsentGivenAt, &snap.ConsentDocVersion,
			&snap.ShareRevokedAt,
			&snap.PromoCodeID, &snap.OriginalAmountKopecks, &snap.DiscountKopecks, &snap.ReferralCode,
			&snap.CreatedAt, &snap.UpdatedAt, &snap.PaidAt, &snap.CompletedAt,
			&snap.DemoStatus, &snap.DemoURL, &snap.DemoAccountID,
			&demoClipsJSON,
			&snap.DemoInvoiceID, &snap.DemoAmountKopecks, &snap.DemoPaymentStatus,
		); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		if err := unmarshalDemoClips(demoClipsJSON, &snap); err != nil {
			return nil, err
		}
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return snaps, nil
}

// unmarshalDemoClips разбирает JSONB-колонку demo_clips в []Track. Пустой/нулевой
// JSON (NULL, '[]', 'null') трактуется как отсутствие клипов — без ошибки.
func unmarshalDemoClips(raw []byte, snap *domain.OrderSnapshot) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &snap.DemoClips); err != nil {
		return fmt.Errorf("unmarshal demo clips: %w", err)
	}
	return nil
}
