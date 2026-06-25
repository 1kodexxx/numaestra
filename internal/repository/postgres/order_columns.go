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
	created_at, updated_at, paid_at, completed_at`
