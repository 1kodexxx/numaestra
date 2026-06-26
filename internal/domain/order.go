package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// PaymentStatus - статус оплаты заказа через Robokassa.
type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusPaid     PaymentStatus = "paid"
	PaymentStatusFailed   PaymentStatus = "failed"
	PaymentStatusRefunded PaymentStatus = "refunded"
)

// GenerationStatus - статус генерации музыки в Suno для данного заказа.
type GenerationStatus string

const (
	GenerationStatusNew        GenerationStatus = "new"        // создан, ждёт оплаты
	GenerationStatusQueued     GenerationStatus = "queued"     // оплачен, в очереди Asynq
	GenerationStatusProcessing GenerationStatus = "processing" // аккаунт захвачен, идёт генерация
	GenerationStatusCompleted  GenerationStatus = "completed"  // все треки получены
	GenerationStatusFailed     GenerationStatus = "failed"     // отказ после всех ретраев
)

// DemoStatus — состояние бесплатного демо-фрагмента (отдельная от оплаты полоса).
// Демо генерируется ДО оплаты и не влияет на payment_status/generation_status.
type DemoStatus string

const (
	DemoStatusNone       DemoStatus = "none"       // демо не запрашивалось
	DemoStatusProcessing DemoStatus = "processing" // демо генерируется
	DemoStatusReady      DemoStatus = "ready"      // демо готово (demoURL заполнен)
	DemoStatusFailed     DemoStatus = "failed"     // демо не удалось (можно повторить)
)

// GenerationPhase — детальная стадия пайплайна (для прогресс-бара на странице статуса).
type GenerationPhase string

const (
	GenerationPhaseQueued     GenerationPhase = "queued"
	GenerationPhasePreparing  GenerationPhase = "preparing"
	GenerationPhaseLyrics     GenerationPhase = "lyrics"
	GenerationPhaseSubmitting GenerationPhase = "submitting"
	GenerationPhaseGenerating GenerationPhase = "generating"
	GenerationPhaseUploading  GenerationPhase = "uploading"
	GenerationPhaseCompleted  GenerationPhase = "completed"
)

// MaxBriefLength — максимально допустимая длина технического задания (в рунах).
// Ограничивает объём данных, который уходит в LLM и в БД: слишком длинный бриф
// раздувает стоимость генерации и нагрузку на хранилище.
const MaxBriefLength = 5000

var (
	ErrBriefTooLong = errors.New("техническое задание слишком длинное")

	ErrOrderNotFound               = errors.New("заказ не найден")
	ErrOrderAlreadyPaid            = errors.New("заказ уже оплачен")
	ErrOrderNotPaid                = errors.New("заказ не оплачен")
	ErrInvalidGenerationTransition = errors.New("недопустимый переход статуса генерации")
	ErrInvalidPaymentTransition    = errors.New("недопустимый переход статуса оплаты")
	ErrOrderUnauthorized           = errors.New("неверный или отсутствующий токен доступа")
	ErrOrderAccessDenied           = errors.New("нет доступа к этому заказу")
	ErrShareNotAvailable           = errors.New("публичная ссылка доступна только для завершённых заказов")
)

// Track - один из сгенерированных вариантов песни (обычно 4 версии на заказ).
type Track struct {
	ID          uuid.UUID
	Index       int    // порядковый номер версии (1..4)
	AudioURL    string // ссылка на объект в S3 после выгрузки
	DurationSec int
	SunoTrackID string // идентификатор трека на стороне Suno, для трассировки
}

// Order - агрегат заказа на создание песни.
type Order struct {
	id        uuid.UUID
	invoiceID int64 // InvId, передаваемый в Robokassa

	customerEmail string
	customerPhone string

	// brief - техническое задание клиента на естественном языке,
	// например: "День рождения дяди Коли, душевная песня в стиле шансон".
	brief string

	// categoryID — идентификатор категории квиза (wedding, birthday и т.д.).
	// Пустая строка означает "свободный" заказ без категории.
	categoryID string

	// sunoPrompt — готовый промпт для Suno API, сформированный из шаблона
	// категории после подстановки ответов пользователя на вопросы квиза.
	// Пустая строка означает, что промпт не был сформирован через квиз —
	// воркер использует brief напрямую.
	sunoPrompt string

	amountKopecks int64 // сумма в копейках - без float, для точности расчётов
	currency      string

	paymentStatus    PaymentStatus
	generationStatus GenerationStatus

	generationPhase    GenerationPhase
	generationProgress int
	tracksReady        int

	assignedAccountID *uuid.UUID
	tracks            []Track
	failureReason     string

	// accessToken — криптостойкий токен выданный клиенту при создании заказа.
	// Предъявляется в заголовке X-Access-Token для доступа к данным заказа.
	accessToken string

	// adminFeedback — последнее сообщение администратора клиенту по этому заказу
	// (отправляется на email и хранится для истории). Пустая строка — обратной
	// связи не было.
	adminFeedback   string
	adminFeedbackAt *time.Time

	consentGivenAt    *time.Time
	consentDocVersion string

	shareRevokedAt *time.Time

	// Промокод и реферал.
	promoCodeID           *uuid.UUID
	originalAmountKopecks int64 // сумма до скидки (0 если промокод не применён)
	discountKopecks       int64
	referralCode          string

	createdAt   time.Time
	updatedAt   time.Time
	paidAt      *time.Time
	completedAt *time.Time

	// Демо-фрагмент (бесплатный, до оплаты). Отдельная полоса, не пересекается
	// с платёжной генерацией: пишется только через демо-поток.
	demoStatus DemoStatus
	demoURL    string
	// demoAccountID — аккаунт, чей слот захвачен под текущее демо (processing).
	// Отдельно от assignedAccountID платного потока. nil вне processing.
	demoAccountID *uuid.UUID
}

// NewOrder создаёт новый заказ в статусе "ожидает оплаты".
func NewOrder(invoiceID int64, customerEmail, customerPhone, brief, categoryID, sunoPrompt, consentDocVersion string, amountKopecks int64) (*Order, error) {
	if customerEmail == "" {
		return nil, errors.New("email клиента обязателен")
	}
	if brief == "" {
		return nil, errors.New("техническое задание на песню не может быть пустым")
	}
	if utf8.RuneCountInString(brief) > MaxBriefLength {
		return nil, ErrBriefTooLong
	}
	if amountKopecks <= 0 {
		return nil, errors.New("сумма заказа должна быть положительной")
	}
	if err := ValidateConsentDocVersion(consentDocVersion); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	consentAt := now

	token, err := generateAccessToken()
	if err != nil {
		return nil, fmt.Errorf("генерация токена доступа: %w", err)
	}

	return &Order{
		id:                uuid.New(),
		invoiceID:         invoiceID,
		customerEmail:     customerEmail,
		customerPhone:     customerPhone,
		brief:             brief,
		categoryID:        categoryID,
		sunoPrompt:        sunoPrompt,
		amountKopecks:     amountKopecks,
		currency:          "RUB",
		paymentStatus:     PaymentStatusPending,
		generationStatus:  GenerationStatusNew,
		demoStatus:        DemoStatusNone,
		accessToken:       token,
		consentGivenAt:    &consentAt,
		consentDocVersion: consentDocVersion,
		createdAt:         now,
		updatedAt:         now,
	}, nil
}

// generateAccessToken создаёт криптостойкий токен из 32 случайных байт (64 hex-символа).
func generateAccessToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// OrderSnapshot - сырые данные заказа из хранилища для восстановления агрегата.
type OrderSnapshot struct {
	ID                    uuid.UUID
	InvoiceID             int64
	CustomerEmail         string
	CustomerPhone         string
	Brief                 string
	CategoryID            string
	SunoPrompt            string
	AmountKopecks         int64
	Currency              string
	PaymentStatus         PaymentStatus
	GenerationStatus      GenerationStatus
	GenerationPhase       GenerationPhase
	GenerationProgress    int
	TracksReady           int
	AssignedAccountID     *uuid.UUID
	Tracks                []Track
	FailureReason         string
	AccessToken           string
	AdminFeedback         string
	AdminFeedbackAt       *time.Time
	ConsentGivenAt        *time.Time
	ConsentDocVersion     string
	ShareRevokedAt        *time.Time
	PromoCodeID           *uuid.UUID
	OriginalAmountKopecks int64
	DiscountKopecks       int64
	ReferralCode          string
	CreatedAt             time.Time
	UpdatedAt             time.Time
	PaidAt                *time.Time
	CompletedAt           *time.Time
	DemoStatus            DemoStatus
	DemoURL               string
	DemoAccountID         *uuid.UUID
}

// RestoreOrder восстанавливает агрегат из снапшота хранилища.
func RestoreOrder(s OrderSnapshot) *Order {
	return &Order{
		id: s.ID, invoiceID: s.InvoiceID,
		customerEmail: s.CustomerEmail, customerPhone: s.CustomerPhone, brief: s.Brief,
		categoryID: s.CategoryID, sunoPrompt: s.SunoPrompt,
		amountKopecks: s.AmountKopecks, currency: s.Currency,
		paymentStatus: s.PaymentStatus, generationStatus: s.GenerationStatus,
		generationPhase: s.GenerationPhase, generationProgress: s.GenerationProgress, tracksReady: s.TracksReady,
		assignedAccountID: s.AssignedAccountID, tracks: s.Tracks, failureReason: s.FailureReason,
		accessToken:           s.AccessToken,
		adminFeedback:         s.AdminFeedback,
		adminFeedbackAt:       s.AdminFeedbackAt,
		consentGivenAt:        s.ConsentGivenAt,
		consentDocVersion:     s.ConsentDocVersion,
		shareRevokedAt:        s.ShareRevokedAt,
		promoCodeID:           s.PromoCodeID,
		originalAmountKopecks: s.OriginalAmountKopecks,
		discountKopecks:       s.DiscountKopecks,
		referralCode:          s.ReferralCode,
		createdAt:             s.CreatedAt, updatedAt: s.UpdatedAt, paidAt: s.PaidAt, completedAt: s.CompletedAt,
		demoStatus: normalizeDemoStatus(s.DemoStatus), demoURL: s.DemoURL, demoAccountID: s.DemoAccountID,
	}
}

// normalizeDemoStatus защищает от пустого значения у старых заказов (до миграции
// колонка отсутствовала) — трактуем пустую строку как «демо не запрашивалось».
func normalizeDemoStatus(s DemoStatus) DemoStatus {
	if s == "" {
		return DemoStatusNone
	}
	return s
}

// --- Геттеры ---

func (o *Order) ID() uuid.UUID                      { return o.id }
func (o *Order) InvoiceID() int64                   { return o.invoiceID }
func (o *Order) CustomerEmail() string              { return o.customerEmail }
func (o *Order) CustomerPhone() string              { return o.customerPhone }
func (o *Order) Brief() string                      { return o.brief }
func (o *Order) CategoryID() string                 { return o.categoryID }
func (o *Order) SunoPrompt() string                 { return o.sunoPrompt }
func (o *Order) AmountKopecks() int64               { return o.amountKopecks }
func (o *Order) Currency() string                   { return o.currency }
func (o *Order) PaymentStatus() PaymentStatus       { return o.paymentStatus }
func (o *Order) GenerationStatus() GenerationStatus { return o.generationStatus }
func (o *Order) GenerationPhase() GenerationPhase   { return o.generationPhase }
func (o *Order) GenerationProgress() int            { return o.generationProgress }
func (o *Order) TracksReady() int                   { return o.tracksReady }
func (o *Order) AssignedAccountID() *uuid.UUID      { return o.assignedAccountID }
func (o *Order) Tracks() []Track                    { return o.tracks }
func (o *Order) FailureReason() string              { return o.failureReason }
func (o *Order) AccessToken() string                { return o.accessToken }
func (o *Order) AdminFeedback() string              { return o.adminFeedback }
func (o *Order) AdminFeedbackAt() *time.Time        { return o.adminFeedbackAt }
func (o *Order) ConsentGivenAt() *time.Time         { return o.consentGivenAt }
func (o *Order) ConsentDocVersion() string          { return o.consentDocVersion }
func (o *Order) ShareRevoked() bool                 { return o.shareRevokedAt != nil }
func (o *Order) ShareRevokedAt() *time.Time         { return o.shareRevokedAt }
func (o *Order) PromoCodeID() *uuid.UUID            { return o.promoCodeID }
func (o *Order) OriginalAmountKopecks() int64       { return o.originalAmountKopecks }
func (o *Order) DiscountKopecks() int64             { return o.discountKopecks }
func (o *Order) ReferralCode() string               { return o.referralCode }
func (o *Order) CreatedAt() time.Time               { return o.createdAt }
func (o *Order) UpdatedAt() time.Time               { return o.updatedAt }
func (o *Order) PaidAt() *time.Time                 { return o.paidAt }
func (o *Order) DemoStatus() DemoStatus             { return o.demoStatus }
func (o *Order) DemoURL() string                    { return o.demoURL }
func (o *Order) DemoAccountID() *uuid.UUID          { return o.demoAccountID }

// SameCustomer сообщает, что два заказа принадлежат одному клиенту (один email
// или один телефон). Используется для доступа к «соседним» заказам по любому
// валидному токену владельца.
func SameCustomer(a, b *Order) bool {
	if a == nil || b == nil {
		return false
	}
	ae := strings.TrimSpace(strings.ToLower(a.CustomerEmail()))
	be := strings.TrimSpace(strings.ToLower(b.CustomerEmail()))
	if ae != "" && ae == be {
		return true
	}
	ap := strings.TrimSpace(a.CustomerPhone())
	bp := strings.TrimSpace(b.CustomerPhone())
	return ap != "" && ap == bp
}

// NormalizeCustomerEmail приводит email к каноническому виду для сравнения.
func NormalizeCustomerEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

// CustomerEmailMatches проверяет, что email совпадает с email заказа.
func CustomerEmailMatches(order *Order, email string) bool {
	if order == nil {
		return false
	}
	oe := NormalizeCustomerEmail(order.CustomerEmail())
	ie := NormalizeCustomerEmail(email)
	return oe != "" && oe == ie
}

// --- Стейт-машина оплаты ---

// MarkPaid фиксирует успешную оплату по уведомлению Robokassa (ResultURL).
func (o *Order) MarkPaid() error {
	if o.paymentStatus == PaymentStatusPaid {
		return ErrOrderAlreadyPaid
	}
	if o.paymentStatus != PaymentStatusPending {
		return ErrInvalidPaymentTransition
	}
	o.paymentStatus = PaymentStatusPaid
	now := time.Now().UTC()
	o.paidAt = &now
	o.touch()
	return nil
}

// MarkPaymentFailed помечает заказ как неоплаченный.
func (o *Order) MarkPaymentFailed() error {
	if o.paymentStatus != PaymentStatusPending {
		return ErrInvalidPaymentTransition
	}
	o.paymentStatus = PaymentStatusFailed
	o.touch()
	return nil
}

// MarkRefunded фиксирует возврат денег клиенту.
func (o *Order) MarkRefunded() error {
	if o.paymentStatus != PaymentStatusPaid {
		return ErrInvalidPaymentTransition
	}
	o.paymentStatus = PaymentStatusRefunded
	o.touch()
	return nil
}

// CancelGenerationForRefund прерывает фоновую генерацию при оформлении возврата,
// чтобы не продолжать тратить кредиты Suno/хранилище S3 на заказ, за который уже
// вернули деньги. Применимо только к queued/processing — для completed заказ уже
// выполнен (трек отдан клиенту до возврата), для failed/new генерации уже нет.
// Возвращает ID захваченного аккаунта (если был), чтобы вызывающий код освободил
// его слот — этот метод сам слот не освобождает, так как Order не знает об
// аккаунтах за пределами их ID.
func (o *Order) CancelGenerationForRefund() *uuid.UUID {
	switch o.generationStatus {
	case GenerationStatusQueued, GenerationStatusProcessing:
	default:
		return nil
	}
	accountID := o.assignedAccountID
	o.generationStatus = GenerationStatusFailed
	o.failureReason = "Заказ возвращён администратором — генерация отменена"
	o.assignedAccountID = nil
	o.touch()
	return accountID
}

// SetAdminFeedback фиксирует сообщение администратора клиенту по заказу.
// Не часть стейт-машины оплаты/генерации — это просто аннотация для истории
// переписки, доступная в любом статусе заказа.
func (o *Order) SetAdminFeedback(message string) error {
	if strings.TrimSpace(message) == "" {
		return errors.New("сообщение обратной связи не может быть пустым")
	}
	now := time.Now().UTC()
	o.adminFeedback = message
	o.adminFeedbackAt = &now
	o.touch()
	return nil
}

// --- Стейт-машина генерации ---

// Enqueue переводит оплаченный заказ в очередь на генерацию.
func (o *Order) Enqueue() error {
	if o.paymentStatus != PaymentStatusPaid {
		return ErrOrderNotPaid
	}
	if o.generationStatus != GenerationStatusNew {
		return ErrInvalidGenerationTransition
	}
	o.generationStatus = GenerationStatusQueued
	o.UpdateGenerationProgress(GenerationPhaseQueued, 3, 0)
	o.touch()
	return nil
}

// UpdateGenerationProgress обновляет детальный прогресс (монотонно — только вперёд).
func (o *Order) UpdateGenerationProgress(phase GenerationPhase, progress, tracksReady int) {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	if tracksReady < 0 {
		tracksReady = 0
	}
	if tracksReady > DefaultTrackCount {
		tracksReady = DefaultTrackCount
	}
	if progress < o.generationProgress && phase == o.generationPhase {
		return
	}
	if phase != "" {
		o.generationPhase = phase
	}
	if progress >= o.generationProgress {
		o.generationProgress = progress
	}
	if tracksReady >= o.tracksReady {
		o.tracksReady = tracksReady
	}
}

// StartProcessing фиксирует захват конкретного Suno-аккаунта под этот заказ.
func (o *Order) StartProcessing(accountID uuid.UUID) error {
	if o.generationStatus != GenerationStatusQueued {
		return ErrInvalidGenerationTransition
	}
	o.generationStatus = GenerationStatusProcessing
	o.assignedAccountID = &accountID
	o.UpdateGenerationProgress(GenerationPhasePreparing, 10, 0)
	o.touch()
	return nil
}

// Complete фиксирует успешное получение всех треков от Suno.
func (o *Order) Complete(tracks []Track) error {
	if o.generationStatus != GenerationStatusProcessing {
		return ErrInvalidGenerationTransition
	}
	if len(tracks) == 0 {
		return errors.New("список треков не может быть пустым при завершении заказа")
	}
	o.generationStatus = GenerationStatusCompleted
	o.tracks = tracks
	o.UpdateGenerationProgress(GenerationPhaseCompleted, 100, len(tracks))
	now := time.Now().UTC()
	o.completedAt = &now
	o.touch()
	return nil
}

// Fail фиксирует окончательный отказ генерации после исчерпания всех ретраев.
func (o *Order) Fail(reason string) error {
	if o.generationStatus == GenerationStatusCompleted {
		return ErrInvalidGenerationTransition
	}
	o.generationStatus = GenerationStatusFailed
	o.failureReason = reason
	o.touch()
	return nil
}

// Regenerate возвращает оплаченный, но упавший в генерации заказ обратно в очередь
// (повторная попытка по решению администратора). Доступна только для paid + failed:
// клиент уже заплатил, но трек не получил — справедливо запустить генерацию заново.
func (o *Order) Regenerate() error {
	if o.paymentStatus != PaymentStatusPaid {
		return ErrOrderNotPaid
	}
	if o.generationStatus != GenerationStatusFailed {
		return ErrInvalidGenerationTransition
	}
	o.generationStatus = GenerationStatusQueued
	o.assignedAccountID = nil
	o.failureReason = ""
	o.generationPhase = GenerationPhaseQueued
	o.generationProgress = 3
	o.tracksReady = 0
	o.touch()
	return nil
}

// RequeueForRetry возвращает заказ в очередь после неуспешной попытки на конкретном
// аккаунте (например, аккаунт ушёл в Cooldown), снимая привязку к аккаунту.
func (o *Order) RequeueForRetry() error {
	if o.generationStatus != GenerationStatusProcessing {
		return ErrInvalidGenerationTransition
	}
	o.generationStatus = GenerationStatusQueued
	o.assignedAccountID = nil
	o.generationPhase = GenerationPhaseQueued
	o.generationProgress = 3
	o.tracksReady = 0
	o.touch()
	return nil
}

// RevokeShare отзывает публичную share-ссылку (/s/{id}). Доступно только для завершённых заказов.
func (o *Order) RevokeShare() error {
	if o.generationStatus != GenerationStatusCompleted {
		return ErrShareNotAvailable
	}
	now := time.Now().UTC()
	o.shareRevokedAt = &now
	o.touch()
	return nil
}

// RestoreShare снова открывает публичную share-ссылку.
func (o *Order) RestoreShare() error {
	if o.generationStatus != GenerationStatusCompleted {
		return ErrShareNotAvailable
	}
	o.shareRevokedAt = nil
	o.touch()
	return nil
}

func (o *Order) touch() {
	o.updatedAt = time.Now().UTC()
}

// QueuePublisher - контракт для постановки задач в очередь (реализация поверх Asynq).
// Передаются только идентификаторы агрегатов, а не их данные: это исключает
// рассинхронизацию состояния между моментом постановки задачи и её обработкой воркером.
type QueuePublisher interface {
	// EnqueueGenerationTask ставит задачу на генерацию музыки для указанного заказа.
	EnqueueGenerationTask(ctx context.Context, orderID uuid.UUID) error

	// EnqueueStatusCheckTask ставит задачу на опрос статуса генерации в Suno (polling).
	EnqueueStatusCheckTask(ctx context.Context, orderID uuid.UUID, sunoJobID string) error

	// EnqueueDemoTask ставит задачу генерации бесплатного демо (очередь "demo").
	EnqueueDemoTask(ctx context.Context, orderID uuid.UUID) error

	// EnqueueDemoCheckTask ставит задачу опроса демо. account_id нужен для
	// освобождения захваченного слота аккаунта по завершении (демо не хранит его
	// на агрегате заказа).
	EnqueueDemoCheckTask(ctx context.Context, orderID uuid.UUID, sunoJobID string, accountID uuid.UUID) error
}

func (o *Order) Snapshot() OrderSnapshot {
	return OrderSnapshot{
		ID:                    o.id,
		InvoiceID:             o.invoiceID,
		CustomerEmail:         o.customerEmail,
		CustomerPhone:         o.customerPhone,
		Brief:                 o.brief,
		CategoryID:            o.categoryID,
		SunoPrompt:            o.sunoPrompt,
		AmountKopecks:         o.amountKopecks,
		Currency:              o.currency,
		PaymentStatus:         o.paymentStatus,
		GenerationStatus:      o.generationStatus,
		GenerationPhase:       o.generationPhase,
		GenerationProgress:    o.generationProgress,
		TracksReady:           o.tracksReady,
		AssignedAccountID:     o.assignedAccountID,
		Tracks:                o.tracks,
		FailureReason:         o.failureReason,
		AccessToken:           o.accessToken,
		AdminFeedback:         o.adminFeedback,
		AdminFeedbackAt:       o.adminFeedbackAt,
		ConsentGivenAt:        o.consentGivenAt,
		ConsentDocVersion:     o.consentDocVersion,
		ShareRevokedAt:        o.shareRevokedAt,
		PromoCodeID:           o.promoCodeID,
		OriginalAmountKopecks: o.originalAmountKopecks,
		DiscountKopecks:       o.discountKopecks,
		ReferralCode:          o.referralCode,
		CreatedAt:             o.createdAt,
		UpdatedAt:             o.updatedAt,
		PaidAt:                o.paidAt,
		CompletedAt:           o.completedAt,
		DemoStatus:            o.demoStatus,
		DemoURL:               o.demoURL,
		DemoAccountID:         o.demoAccountID,
	}
}

// --- Демо-фрагмент (отдельная полоса, не пересекается с оплатой/генерацией) ---

// StartDemo помечает начало генерации демо и фиксирует захваченный аккаунт.
// Допустимо только из none/failed — защищает от повторного запуска уже идущего/
// готового демо.
func (o *Order) StartDemo(accountID uuid.UUID) error {
	if o.demoStatus == DemoStatusProcessing {
		return errors.New("демо уже генерируется")
	}
	if o.demoStatus == DemoStatusReady {
		return errors.New("демо уже готово")
	}
	o.demoStatus = DemoStatusProcessing
	o.demoAccountID = &accountID
	o.touch()
	return nil
}

// CompleteDemo фиксирует готовое демо и снимает привязку к аккаунту (слот освобождён).
func (o *Order) CompleteDemo(url string) error {
	if url == "" {
		return errors.New("demo url не может быть пустым")
	}
	o.demoStatus = DemoStatusReady
	o.demoURL = url
	o.demoAccountID = nil
	o.touch()
	return nil
}

// FailDemo помечает демо как неуспешное (можно повторить) и снимает привязку к аккаунту.
func (o *Order) FailDemo() {
	o.demoStatus = DemoStatusFailed
	o.demoAccountID = nil
	o.touch()
}

// ApplyPromo применяет скидку промокода к заказу.
func (o *Order) ApplyPromo(promoID uuid.UUID, discountKopecks int64) {
	o.originalAmountKopecks = o.amountKopecks
	o.promoCodeID = &promoID
	o.discountKopecks = discountKopecks
	o.amountKopecks -= discountKopecks
	if o.amountKopecks < 0 {
		o.amountKopecks = 0
	}
}

// SetReferralCode фиксирует реферальный код, по которому пришёл клиент.
func (o *Order) SetReferralCode(code string) {
	o.referralCode = code
}

// OrderRepository - контракт для персистентности заказов.
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
	GetByInvoiceID(ctx context.Context, invoiceID int64) (*Order, error)
	Update(ctx context.Context, order *Order) error

	// ApplyPaymentSuccess идемпотентно и атомарно фиксирует успешную оплату:
	// переводит заказ в paid+queued ТОЛЬКО если он ещё в payment_status='pending'.
	// Возвращает applied=true, если переход выполнен именно этим вызовом, и
	// applied=false, если заказ уже был оплачен (например, параллельной доставкой
	// вебхука). Это защищает от гонки двойной генерации без отдельной version-колонки.
	ApplyPaymentSuccess(ctx context.Context, order *Order) (applied bool, err error)
	ListByCustomerEmail(ctx context.Context, email string, limit, offset int) ([]*Order, error)
	ListByCustomerPhone(ctx context.Context, phone string, limit, offset int) ([]*Order, error)

	// NextInvoiceID возвращает следующий InvId из PostgreSQL sequence invoice_id_seq.
	// Гарантирует уникальность даже при нескольких инстансах сервиса.
	NextInvoiceID(ctx context.Context) (int64, error)

	// GetByAccessToken находит заказ по токену доступа клиента.
	// Используется middleware аутентификации для проверки X-Access-Token.
	GetByAccessToken(ctx context.Context, token string) (*Order, error)

	// SetAdminFeedback сохраняет сообщение администратора клиенту по заказу
	// (история переписки — заказ может получить несколько сообщений со временем,
	// здесь хранится последнее; письмо отправляется отдельно через notify.Notifier).
	SetAdminFeedback(ctx context.Context, id uuid.UUID, feedback string, at time.Time) error

	// ListAll возвращает страницу всех заказов (Admin API), отсортированных по дате убыванием.
	ListAll(ctx context.Context, limit, offset int) ([]*Order, error)
	// CountAll возвращает общее число заказов (для пагинации Admin API).
	CountAll(ctx context.Context) (int, error)

	// ListStuckProcessing возвращает заказы, застрявшие в статусе processing
	// дольше порогового времени (recovery после падения пода/воркера).
	ListStuckProcessing(ctx context.Context, olderThan time.Time) ([]*Order, error)

	// ListStuckQueued возвращает оплаченные заказы, застрявшие в статусе queued
	// дольше порогового времени: задача генерации потеряна или исчерпала ретраи
	// (ушла в archived) и заказ навсегда завис в «В очереди». Recovery повторно
	// ставит для них задачу генерации.
	ListStuckQueued(ctx context.Context, olderThan time.Time) ([]*Order, error)

	// ListPendingPayment возвращает неоплаченные (pending) заказы, созданные в окне
	// [createdAfter, createdBefore]. Фоновая сверка платежей опрашивает по ним
	// Robokassa (OpStateExt) и активирует те, что фактически оплачены, но чей вебхук
	// ResultURL не дошёл (IP-allowlist, сетевой сбой) при не вернувшемся на страницу
	// клиенте. Нижняя граница окна отсекает свежие заказы (ещё на странице оплаты),
	// верхняя — выход за платёжное окно.
	ListPendingPayment(ctx context.Context, createdAfter, createdBefore time.Time) ([]*Order, error)

	// UpdateDemo пишет только demo-колонки (status/url/account_id), не затрагивая
	// платёжные. Изолирует демо-полосу от платёжного Update (исключает взаимное
	// затирание).
	UpdateDemo(ctx context.Context, order *Order) error

	// ListStuckDemo возвращает заказы с демо в processing дольше порога — для
	// освобождения захваченного слота аккаунта при крахе демо-задачи.
	ListStuckDemo(ctx context.Context, olderThan time.Time) ([]*Order, error)

	// Delete безвозвратно удаляет заказ и связанные треки (CASCADE в БД).
	Delete(ctx context.Context, id uuid.UUID) error
}

// OrderTrackS3Key возвращает ключ MP3-трека заказа в объектном хранилище.
func OrderTrackS3Key(orderID uuid.UUID, index int) string {
	return fmt.Sprintf("tracks/%s/%d.mp3", orderID, index)
}

// TrackStorage - порт для долгосрочного хранения готовых треков.
// Домен не знает, с каким конкретным хранилищем работает реализация: S3, Yandex Cloud, MinIO.
type TrackStorage interface {
	// UploadFromURL скачивает файл по sourceURL и сохраняет под ключом key.
	// Возвращает постоянную публичную ссылку на объект в хранилище.
	UploadFromURL(ctx context.Context, sourceURL, key, contentType string) (publicURL string, err error)
	// Upload сохраняет готовые байты под ключом key. Используется для демо-фрагмента,
	// собранного из обрезанного и обработанного аудио.
	Upload(ctx context.Context, key, contentType string, data []byte) (publicURL string, err error)
	// DeleteOrderTracks удаляет все MP3-объекты заказа по шаблону tracks/{id}/{n}.mp3.
	DeleteOrderTracks(ctx context.Context, orderID uuid.UUID) error
}
