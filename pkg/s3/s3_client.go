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
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client загружает объекты в S3-совместимое хранилище.
type Client struct {
	endpoint   string // например "https://s3.amazonaws.com" или "https://storage.yandexcloud.net"
	region     string
	bucket     string
	accessKey  string
	secretKey  string
	httpClient *http.Client
}

// New создаёт S3-клиент. endpoint — базовый URL хранилища без имени бакета.
func New(endpoint, region, bucket, accessKey, secretKey string) *Client {
	return &Client{
		endpoint:   strings.TrimRight(endpoint, "/"),
		region:     region,
		bucket:     bucket,
		accessKey:  accessKey,
		secretKey:  secretKey,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// UploadFromURL скачивает файл по sourceURL и загружает его в S3 под ключом key.
// Возвращает постоянную публичную ссылку на объект в бакете.
//
// key — путь внутри бакета, например "tracks/order-uuid/1.mp3".
// contentType — MIME-тип файла, например "audio/mpeg".
func (c *Client) UploadFromURL(ctx context.Context, sourceURL, key, contentType string) (string, error) {
	// 1. Скачиваем аудио с временной ссылки Suno.
	body, err := c.download(ctx, sourceURL)
	if err != nil {
		return "", fmt.Errorf("скачивание трека с %s: %w", sourceURL, err)
	}
	defer body.Close()

	// 2. Загружаем в S3.
	if err := c.put(ctx, key, contentType, body); err != nil {
		return "", fmt.Errorf("загрузка в S3 (key=%s): %w", key, err)
	}

	// 3. Формируем постоянную ссылку.
	publicURL := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)
	return publicURL, nil
}

// download скачивает тело ответа по URL, возвращает ReadCloser для потоковой передачи.
func (c *Client) download(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d при скачивании", resp.StatusCode)
	}
	return resp.Body, nil
}

// put выполняет PUT-запрос к S3 с AWS Signature V4.
// body читается потоково — не буферизуется в памяти целиком.
func (c *Client) put(ctx context.Context, key, contentType string, body io.Reader) error {
	// Читаем тело в буфер — нужен для вычисления SHA256 и Content-Length.
	// Треки Suno обычно 3–5 МБ, это приемлемо.
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("чтение тела: %w", err)
	}

	objectURL := fmt.Sprintf("%s/%s/%s", c.endpoint, c.bucket, key)

	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	payloadHash := hexSHA256(data)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, objectURL, strings.NewReader(string(data)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	req.ContentLength = int64(len(data))

	// AWS Signature V4.
	authHeader := c.signV4(req, dateStamp, amzDate, payloadHash)
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
