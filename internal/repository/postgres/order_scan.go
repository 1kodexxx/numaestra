package postgres

import (
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/numaestra/numaestra/internal/domain"
)

func scanOrderSnapshot(row pgx.Row) (domain.OrderSnapshot, error) {
	var snap domain.OrderSnapshot
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
	)
	if err != nil {
		return domain.OrderSnapshot{}, fmt.Errorf("scan order: %w", err)
	}
	return snap, nil
}

func scanOrderRows(rows pgx.Rows) ([]domain.OrderSnapshot, error) {
	var snaps []domain.OrderSnapshot
	for rows.Next() {
		var snap domain.OrderSnapshot
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
		); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		snaps = append(snaps, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return snaps, nil
}
