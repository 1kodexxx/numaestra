// Package s3 реализует загрузку файлов в S3-совместимое хранилище.
//
// Использует AWS Signature Version 4 поверх стандартной библиотеки net/http
// без внешних зависимостей. Совместим с AWS S3, Yandex Object Storage, MinIO.
//
// Основной сценарий использования в Numaestra:
//  1. Получить временную ссылку на трек от Suno (протухает через несколько часов).
//  2. Скачать аудио-файл по этой ссылке.
//  3. Загрузить в собственный S3-бакет под постоянным ключом.
//  4. Обновить domain.Track.AudioURL на постоянную ссылку.
package s3

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

// Client загружает объекты в S3-совместимое хранилище.
type Client struct {
	endpoint  string // например "https://s3.amazonaws.com" или "https://storage.yandexcloud.net"
	region    string
	bucket    string
	accessKey string
	secretKey string
	// httpClient — для запросов к самому S3 (PUT). Сюда SSRF-фильтр НЕ применяется:
	// S3-эндпоинт может быть приватным (например MinIO во внутренней сети).
	httpClient *http.Client
	// downloadClient — для скачивания внешних source-URL (треки Suno). Имеет
	// SSRF-guard: запрещает подключения к приватным/loopback/link-local адресам.
	downloadClient *http.Client
}

// New создаёт S3-клиент. endpoint — базовый URL хранилища без имени бакета.
func New(endpoint, region, bucket, accessKey, secretKey string) *Client {
	return &Client{
		endpoint:       strings.TrimRight(endpoint, "/"),
		region:         region,
		bucket:         bucket,
		accessKey:      accessKey,
		secretKey:      secretKey,
		httpClient:     &http.Client{Timeout: 120 * time.Second},
		downloadClient: newGuardedHTTPClient(120 * time.Second),
	}
}

// unsignedPayload — специальное значение x-amz-content-sha256, разрешённое S3
// при работе поверх HTTPS. Позволяет не вычислять SHA256 всего тела заранее и,
// как следствие, не буферизовать весь файл в памяти, а стримить его напрямую.
const unsignedPayload = "UNSIGNED-PAYLOAD"

// UploadFromURL скачивает файл по sourceURL и загружает его в S3 под ключом key.
// Возвращает постоянную публичную ссылку на объект в бакете.
//
// key — путь внутри бакета, например "tracks/order-uuid/1.mp3".
// contentType — MIME-тип файла, например "audio/mpeg".
func (c *Client) UploadFromURL(ctx context.Context, sourceURL, key, contentType string) (string, error) {
	// 1. Скачиваем аудио с временной ссылки Suno.
	body, contentLength, err := c.download(ctx, sourceURL)
	if err != nil {
		return "", fmt.Errorf("скачивание трека с %s: %w", sourceURL, err)
	}
	defer body.Close()

	// 2. Загружаем в S3 потоком, без буферизации всего файла в RAM.
	// Content-Disposition: attachment заставляет браузер скачивать файл (а не
	// проигрывать в табе) при прямом открытии ссылки — это надёжный путь для
	// кнопки «Скачать», даже без CORS на бакете.
	filename := key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		filename = key[i+1:]
	}
	disposition := fmt.Sprintf("attachment; filename=%q", "numaestra-"+filename)
	if err := c.put(ctx, key, contentType, disposition, body, contentLength); err != nil {
		return "", fmt.Errorf("загрузка в S3 (key=%s): %w", key, err)
	}

	// 3. Формируем постоянную ссылку.
	publicURL := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)
	return publicURL, nil
}

// Upload загружает уже готовые байты под ключом key и возвращает постоянную
// публичную ссылку. В отличие от UploadFromURL источник — данные в памяти
// (например, обложка категории, пришедшая из формы админки).
func (c *Client) Upload(ctx context.Context, key, contentType string, data []byte) (string, error) {
	if err := c.put(ctx, key, contentType, "", bytes.NewReader(data), int64(len(data))); err != nil {
		return "", fmt.Errorf("загрузка в S3 (key=%s): %w", key, err)
	}
	return fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key), nil
}

// DeleteOrderTracks удаляет все MP3-объекты заказа (tracks/{id}/1..N.mp3).
// Отсутствие объекта (HTTP 404) не считается ошибкой.
func (c *Client) DeleteOrderTracks(ctx context.Context, orderID uuid.UUID) error {
	for i := 1; i <= domain.DefaultTrackCount; i++ {
		if err := c.delete(ctx, domain.OrderTrackS3Key(orderID, i)); err != nil {
			return err
		}
	}
	return nil
}

// delete выполняет DELETE-запрос к S3 с AWS Signature V4 (пустое тело).
func (c *Client) delete(ctx context.Context, key string) error {
	objectURL := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	payloadHash := hexSHA256(nil)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, objectURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)

	authHeader := c.signV4EmptyBody(req, dateStamp, amzDate, payloadHash)
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}
	respBody, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("S3 DELETE %s: HTTP %d: %s", key, resp.StatusCode, string(respBody))
}

// signV4EmptyBody подписывает запрос без тела (DELETE и аналоги).
func (c *Client) signV4EmptyBody(req *http.Request, dateStamp, amzDate, payloadHash string) string {
	parsedURL, _ := url.Parse(req.URL.String())
	canonicalURI := parsedURL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf(
		"host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		parsedURL.Host,
		payloadHash,
		amzDate,
	)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		"",
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, c.region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(c.secretKey, dateStamp, c.region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	return fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.accessKey, credentialScope, signedHeaders, signature,
	)
}

// isDisallowedIP сообщает, что адрес принадлежит небезопасной для SSRF зоне:
// loopback (127.0.0.1/::1), приватные сети (RFC1918, fc00::/7), link-local
// (включая облачный метадата-эндпоинт 169.254.169.254), unspecified и multicast.
func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}

// newGuardedHTTPClient возвращает http.Client, который при установке КАЖДОГО
// TCP-соединения (в т.ч. после редиректа и уже после DNS-резолвинга, что
// закрывает и DNS-rebinding) запрещает подключения к приватным/служебным
// адресам. Используется для скачивания внешних URL — защита от SSRF.
func newGuardedHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("ssrf-guard: не удалось разобрать адрес %q", host)
			}
			if isDisallowedIP(ip) {
				return fmt.Errorf("ssrf-guard: подключение к небезопасному адресу %s запрещено", ip)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: dialer.DialContext},
	}
}

// download скачивает тело ответа по URL и возвращает ReadCloser для потоковой
// передачи вместе с длиной контента (или -1, если сервер её не сообщил).
// Принимаются только http(s)-схемы; запросы идут через downloadClient с
// SSRF-фильтром.
func (c *Client) download(ctx context.Context, rawURL string) (io.ReadCloser, int64, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, 0, fmt.Errorf("некорректный source URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, 0, fmt.Errorf("недопустимая схема source URL %q (ожидается http/https)", parsed.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.downloadClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, 0, fmt.Errorf("HTTP %d при скачивании", resp.StatusCode)
	}
	return resp.Body, resp.ContentLength, nil
}

// put выполняет PUT-запрос к S3 с AWS Signature V4.
//
// Тело передаётся потоково и не буферизуется целиком в памяти, когда длина
// контента известна (contentLength >= 0): используется x-amz-content-sha256 =
// UNSIGNED-PAYLOAD, что допустимо поверх HTTPS. Если длина неизвестна, делаем
// безопасный fallback — читаем тело в буфер, чтобы выставить корректный
// Content-Length (S3 PUT требует его).
func (c *Client) put(ctx context.Context, key, contentType, contentDisposition string, body io.Reader, contentLength int64) error {
	objectURL := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	reqBody := body
	if contentLength < 0 {
		data, err := io.ReadAll(body)
		if err != nil {
			return fmt.Errorf("чтение тела: %w", err)
		}
		reqBody = strings.NewReader(string(data))
		contentLength = int64(len(data))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, reqBody)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentType)
	// Треки и демо иммутабельны (фиксированный ключ, содержимое не меняется),
	// поэтому кэшируем агрессивно: после первой загрузки браузер/CDN отдают файл
	// мгновенно — без повторных запросов в S3 при перемотке, паузе и повторном
	// воспроизведении. Это убирает ощущаемую задержку загрузки треков.
	// Стандартный несигнируемый заголовок — S3 хранит его и отдаёт при GET.
	req.Header.Set("Cache-Control", "public, max-age=31536000, immutable")
	if contentDisposition != "" {
		// Несигнируемый стандартный заголовок — S3 сохранит его как метаданные
		// объекта и будет отдавать при GET. На подпись SigV4 не влияет.
		req.Header.Set("Content-Disposition", contentDisposition)
	}
	req.Header.Set("x-amz-content-sha256", unsignedPayload)
	req.Header.Set("x-amz-date", amzDate)
	req.ContentLength = contentLength

	// AWS Signature V4 с неподписанным телом (UNSIGNED-PAYLOAD).
	authHeader := c.signV4(req, dateStamp, amzDate, unsignedPayload)
	req.Header.Set("Authorization", authHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 вернул HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// signV4 вычисляет Authorization-заголовок по алгоритму AWS Signature Version 4.
// Документация: https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-auth-using-authorization-header.html
func (c *Client) signV4(req *http.Request, dateStamp, amzDate, payloadHash string) string {
	// 1. Canonical request.
	parsedURL, _ := url.Parse(req.URL.String())
	canonicalURI := parsedURL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf(
		"content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"),
		parsedURL.Host,
		payloadHash,
		amzDate,
	)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		"", // query string
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// 2. String to sign.
	credentialScope := fmt.Sprintf("%s/%s/s3/aws4_request", dateStamp, c.region)
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hexSHA256([]byte(canonicalRequest)),
	}, "\n")

	// 3. Signing key.
	signingKey := deriveSigningKey(c.secretKey, dateStamp, c.region, "s3")

	// 4. Signature.
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	return fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		c.accessKey, credentialScope, signedHeaders, signature,
	)
}

func deriveSigningKey(secretKey, dateStamp, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hexSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
