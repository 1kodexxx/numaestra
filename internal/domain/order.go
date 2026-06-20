package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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

	assignedAccountID *uuid.UUID
	tracks            []Track
	failureReason     string

	// accessToken — криптостойкий токен выданный клиенту при создании заказа.
	// Предъявляется в заголовке X-Access-Token для доступа к данным заказа.
	accessToken string

	createdAt   time.Time
	updatedAt   time.Time
	paidAt      *time.Time
	completedAt *time.Time
}

// NewOrder создаёт новый заказ в статусе "ожидает оплаты".
func NewOrder(invoiceID int64, customerEmail, customerPhone, brief, categoryID, sunoPrompt string, amountKopecks int64) (*Order, error) {
	if customerEmail == "" && customerPhone == "" {
		return nil, errors.New("должен быть указан хотя бы один контакт клиента")
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

	now := time.Now().UTC()

	token, err := generateAccessToken()
	if err != nil {
		return nil, fmt.Errorf("генерация токена доступа: %w", err)
	}

	return &Order{
		id:               uuid.New(),
		invoiceID:        invoiceID,
		customerEmail:    customerEmail,
		customerPhone:    customerPhone,
		brief:            brief,
		categoryID:       categoryID,
		sunoPrompt:       sunoPrompt,
		amountKopecks:    amountKopecks,
		currency:         "RUB",
		paymentStatus:    PaymentStatusPending,
		generationStatus: GenerationStatusNew,
		accessToken:      token,
		createdAt:        now,
		updatedAt:        now,
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
	ID                uuid.UUID
	InvoiceID         int64
	CustomerEmail     string
	CustomerPhone     string
	Brief             string
	CategoryID        string
	SunoPrompt        string
	AmountKopecks     int64
	Currency          string
	PaymentStatus     PaymentStatus
	GenerationStatus  GenerationStatus
	AssignedAccountID *uuid.UUID
	Tracks            []Track
	FailureReason     string
	AccessToken       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	PaidAt            *time.Time
	CompletedAt       *time.Time
}

// RestoreOrder восстанавливает агрегат из снапшота хранилища.
func RestoreOrder(s OrderSnapshot) *Order {
	return &Order{
		id: s.ID, invoiceID: s.InvoiceID,
		customerEmail: s.CustomerEmail, customerPhone: s.CustomerPhone, brief: s.Brief,
		categoryID: s.CategoryID, sunoPrompt: s.SunoPrompt,
		amountKopecks: s.AmountKopecks, currency: s.Currency,
		paymentStatus: s.PaymentStatus, generationStatus: s.GenerationStatus,
		assignedAccountID: s.AssignedAccountID, tracks: s.Tracks, failureReason: s.FailureReason,
		accessToken: s.AccessToken,
		createdAt:   s.CreatedAt, updatedAt: s.UpdatedAt, paidAt: s.PaidAt, completedAt: s.CompletedAt,
	}
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
func (o *Order) AssignedAccountID() *uuid.UUID      { return o.assignedAccountID }
func (o *Order) Tracks() []Track                    { return o.tracks }
func (o *Order) FailureReason() string              { return o.failureReason }
func (o *Order) AccessToken() string                { return o.accessToken }
func (o *Order) UpdatedAt() time.Time               { return o.updatedAt }

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
	o.touch()
	return nil
}

// StartProcessing фиксирует захват конкретного Suno-аккаунта под этот заказ.
func (o *Order) StartProcessing(accountID uuid.UUID) error {
	if o.generationStatus != GenerationStatusQueued {
		return ErrInvalidGenerationTransition
	}
	o.generationStatus = GenerationStatusProcessing
	o.assignedAccountID = &accountID
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

// RequeueForRetry возвращает заказ в очередь после неуспешной попытки на конкретном
// аккаунте (например, аккаунт ушёл в Cooldown), снимая привязку к аккаунту.
func (o *Order) RequeueForRetry() error {
	if o.generationStatus != GenerationStatusProcessing {
		return ErrInvalidGenerationTransition
	}
	o.generationStatus = GenerationStatusQueued
	o.assignedAccountID = nil
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
}

func (o *Order) Snapshot() OrderSnapshot {
	return OrderSnapshot{
		ID:                o.id,
		InvoiceID:         o.invoiceID,
		CustomerEmail:     o.customerEmail,
		CustomerPhone:     o.customerPhone,
		Brief:             o.brief,
		CategoryID:        o.categoryID,
		SunoPrompt:        o.sunoPrompt,
		AmountKopecks:     o.amountKopecks,
		Currency:          o.currency,
		PaymentStatus:     o.paymentStatus,
		GenerationStatus:  o.generationStatus,
		AssignedAccountID: o.assignedAccountID,
		Tracks:            o.tracks,
		FailureReason:     o.failureReason,
		AccessToken:       o.accessToken,
		CreatedAt:         o.createdAt,
		UpdatedAt:         o.updatedAt,
		PaidAt:            o.paidAt,
		CompletedAt:       o.completedAt,
	}
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

	// ListAll возвращает страницу всех заказов (Admin API), отсортированных по дате убыванием.
	ListAll(ctx context.Context, limit, offset int) ([]*Order, error)
	// CountAll возвращает общее число заказов (для пагинации Admin API).
	CountAll(ctx context.Context) (int, error)
}

// TrackStorage - порт для долгосрочного хранения готовых треков.
// Домен не знает, с каким конкретным хранилищем работает реализация: S3, Yandex Cloud, MinIO.
type TrackStorage interface {
	// UploadFromURL скачивает файл по sourceURL и сохраняет под ключом key.
	// Возвращает постоянную публичную ссылку на объект в хранилище.
	UploadFromURL(ctx context.Context, sourceURL, key, contentType string) (publicURL string, err error)
}
