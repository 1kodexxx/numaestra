package postgres

// Единый список колонок orders для SELECT и Scan — без рассинхрона при миграциях.
const orderSelectColumns = `
	id, invoice_id, customer_email, customer_phone, brief,
	COALESCE(category_id, '') AS category_id,
	COALESCE(suno_prompt, '') AS suno_prompt,
	amount_kopecks, currency, payment_status, generation_status,
	generation_phase, generation_progress, tracks_ready,
	assigned_account_id, failure_reason, access_token,
	COALESCE(admin_feedback, '') AS admin_feedback, admin_feedback_at,
	consent_given_at, COALESCE(consent_doc_version, '') AS consent_doc_version,
	share_revoked_at,
	promo_code_id, COALESCE(original_amount_kopecks, 0) AS original_amount_kopecks,
	COALESCE(discount_kopecks, 0) AS discount_kopecks,
	COALESCE(referral_code, '') AS referral_code,
	created_at, updated_at, paid_at, completed_at,
	COALESCE(demo_status, 'none') AS demo_status, COALESCE(demo_url, '') AS demo_url,
	demo_account_id,
	COALESCE(demo_clips, '[]'::jsonb) AS demo_clips`
