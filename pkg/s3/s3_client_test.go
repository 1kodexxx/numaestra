package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/numaestra/numaestra/internal/domain"
)

// allowLoopbackDownloads отключает SSRF-guard у клиента: httptest-серверы
// слушают на 127.0.0.1, который guard в проде справедливо блокирует.
func allowLoopbackDownloads(c *Client) {
	c.downloadClient = http.DefaultClient
}

func TestUploadFromURL_Success(t *testing.T) {
	const audio = "fake-mp3-bytes"
	var putBody string
	var gotAuth, gotDate, gotSHA, gotCache string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Источник (временная ссылка Suno).
			_, _ = w.Write([]byte(audio))
		case http.MethodPut:
			// Целевой S3 PUT.
			b, _ := io.ReadAll(r.Body)
			putBody = string(b)
			gotAuth = r.Header.Get("Authorization")
			gotDate = r.Header.Get("x-amz-date")
			gotSHA = r.Header.Get("x-amz-content-sha256")
			gotCache = r.Header.Get("Cache-Control")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret")
	allowLoopbackDownloads(c)
	sourceURL := srv.URL + "/suno/temp.mp3"
	publicURL, err := c.UploadFromURL(context.Background(), sourceURL, "tracks/order/1.mp3", "audio/mpeg")
	if err != nil {
		t.Fatalf("UploadFromURL упал: %v", err)
	}

	wantURL := srv.URL + "/test-bucket/tracks/order/1.mp3"
	if publicURL != wantURL {
		t.Errorf("публичная ссылка = %q, хотели %q", publicURL, wantURL)
	}
	if putBody != audio {
		t.Errorf("в S3 загружено %q, ожидали %q", putBody, audio)
	}
	if !strings.HasPrefix(gotAuth, "AWS4-HMAC-SHA256 Credential=AKIA/") {
		t.Errorf("Authorization не похож на SigV4: %q", gotAuth)
	}
	if gotDate == "" || gotSHA == "" {
		t.Error("должны быть выставлены заголовки x-amz-date и x-amz-content-sha256")
	}
	if gotCache != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q, ожидали immutable-кэширование", gotCache)
	}
}

func TestUploadFromURL_DownloadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "bucket", "AK", "sk")
	allowLoopbackDownloads(c)
	if _, err := c.UploadFromURL(context.Background(), srv.URL+"/missing.mp3", "k.mp3", "audio/mpeg"); err == nil {
		t.Fatal("ожидали ошибку при недоступном источнике")
	}
}

func TestUploadFromURL_PutFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte("data"))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("access denied"))
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "bucket", "AK", "sk")
	allowLoopbackDownloads(c)
	if _, err := c.UploadFromURL(context.Background(), srv.URL+"/a.mp3", "k.mp3", "audio/mpeg"); err == nil {
		t.Fatal("ожидали ошибку при отказе S3 (403)")
	}
}

func TestUploadFromURL_RejectsLoopbackSource(t *testing.T) {
	// SSRF: source-URL, указывающий на loopback, должен блокироваться guard'ом.
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("secret-internal-data"))
	}))
	defer src.Close()

	c := New("https://s3.example.com", "us-east-1", "bucket", "AK", "sk") // guard активен
	if _, err := c.UploadFromURL(context.Background(), src.URL+"/x.mp3", "k.mp3", "audio/mpeg"); err == nil {
		t.Fatal("ожидали блокировку SSRF при скачивании с loopback-адреса")
	}
}

func TestUploadFromURL_RejectsBadScheme(t *testing.T) {
	c := New("https://s3.example.com", "us-east-1", "bucket", "AK", "sk")
	if _, err := c.UploadFromURL(context.Background(), "file:///etc/passwd", "k.mp3", "audio/mpeg"); err == nil {
		t.Fatal("ожидали отказ для схемы file://")
	}
}

func TestSignV4_Deterministic(t *testing.T) {
	// Один и тот же запрос с фиксированными датами должен давать одинаковую подпись.
	c := New("https://s3.example.com", "us-east-1", "bucket", "AKIA", "secret")
	req1, _ := http.NewRequest(http.MethodPut, "https://s3.example.com/bucket/key", nil)
	req1.Header.Set("Content-Type", "audio/mpeg")
	req2, _ := http.NewRequest(http.MethodPut, "https://s3.example.com/bucket/key", nil)
	req2.Header.Set("Content-Type", "audio/mpeg")

	h := "abc123"
	sig1 := c.signV4(req1, "20240101", "20240101T000000Z", h)
	sig2 := c.signV4(req2, "20240101", "20240101T000000Z", h)
	if sig1 != sig2 {
		t.Error("подпись SigV4 должна быть детерминированной для одинаковых входных данных")
	}
	if !strings.Contains(sig1, "Signature=") {
		t.Errorf("подпись должна содержать Signature=, получили %q", sig1)
	}
}

func TestDeleteOrderTracks_Success(t *testing.T) {
	orderID := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	var deleted []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret")
	if err := c.DeleteOrderTracks(context.Background(), orderID); err != nil {
		t.Fatalf("DeleteOrderTracks: %v", err)
	}
	if len(deleted) != domain.DefaultTrackCount {
		t.Fatalf("ожидали %d DELETE, получили %d: %v", domain.DefaultTrackCount, len(deleted), deleted)
	}
}

func TestDeleteOrderTracks_NotFoundIsOK(t *testing.T) {
	orderID := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret")
	if err := c.DeleteOrderTracks(context.Background(), orderID); err != nil {
		t.Fatalf("404 не должен быть ошибкой: %v", err)
	}
}

func TestDeleteOrderTracks_S3Error(t *testing.T) {
	orderID := uuid.New()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret")
	if err := c.DeleteOrderTracks(context.Background(), orderID); err == nil {
		t.Fatal("ожидали ошибку при HTTP 500 от S3")
	}
}

func TestDeleteByURL_Success(t *testing.T) {
	var deletedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deletedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret")
	url := srv.URL + "/test-bucket/tracks/o/demo-abc.mp3"
	if err := c.DeleteByURL(context.Background(), url); err != nil {
		t.Fatalf("DeleteByURL: %v", err)
	}
	if deletedPath != "/test-bucket/tracks/o/demo-abc.mp3" {
		t.Errorf("удалён неожиданный путь: %s", deletedPath)
	}
}

func TestDeleteByURL_ForeignURLRejected(t *testing.T) {
	c := New("https://storage.example.com", "us-east-1", "test-bucket", "AKIA", "secret")
	// URL чужого бакета/хоста не должен приводить к удалению.
	if err := c.DeleteByURL(context.Background(), "https://evil.example.com/other-bucket/x.mp3"); err == nil {
		t.Fatal("ожидали отказ для URL не из нашего бакета")
	}
}

func TestUploadFromURL_PublicBaseURL_UsesCDNForLinkButS3ForPut(t *testing.T) {
	var gotPut bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte("audio"))
		case http.MethodPut:
			gotPut = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret").
		WithPublicBaseURL("https://cdn.example.com")
	allowLoopbackDownloads(c)

	publicURL, err := c.UploadFromURL(context.Background(), srv.URL+"/src.mp3", "tracks/o/1.mp3", "audio/mpeg")
	if err != nil {
		t.Fatalf("UploadFromURL: %v", err)
	}
	if publicURL != "https://cdn.example.com/tracks/o/1.mp3" {
		t.Errorf("публичная ссылка должна вести на CDN, получили %q", publicURL)
	}
	if !gotPut {
		t.Error("загрузка должна идти на S3-endpoint, а не на CDN")
	}
}

func TestDeleteByURL_HandlesCDNAndS3Origin(t *testing.T) {
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleted = append(deleted, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret").
		WithPublicBaseURL("https://cdn.example.com")

	// Новая ссылка (CDN-домен).
	if err := c.DeleteByURL(context.Background(), "https://cdn.example.com/demos/o.mp3"); err != nil {
		t.Fatalf("удаление по CDN-ссылке: %v", err)
	}
	// Старая ссылка (прямой S3-origin) — до подключения CDN.
	if err := c.DeleteByURL(context.Background(), srv.URL+"/test-bucket/tracks/o/demo-x.mp3"); err != nil {
		t.Fatalf("удаление по старой S3-ссылке: %v", err)
	}

	if len(deleted) != 2 {
		t.Fatalf("ожидали 2 DELETE на S3-endpoint, получили %d: %v", len(deleted), deleted)
	}
	if deleted[0] != "/test-bucket/demos/o.mp3" {
		t.Errorf("ключ из CDN-ссылки извлечён неверно: %s", deleted[0])
	}
	if deleted[1] != "/test-bucket/tracks/o/demo-x.mp3" {
		t.Errorf("ключ из S3-ссылки извлечён неверно: %s", deleted[1])
	}
}

func TestPresignGetURL_ContainsSignatureAndTTL(t *testing.T) {
	c := New("https://s3.example.com", "us-east-1", "test-bucket", "AKIA", "secret")
	u, err := c.PresignGetURL(context.Background(), "tracks/o/1.mp3", 24*time.Hour)
	if err != nil {
		t.Fatalf("PresignGetURL: %v", err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("результат не является URL: %v", err)
	}
	// path-style на S3-endpoint: /{bucket}/{key}.
	if parsed.Path != "/test-bucket/tracks/o/1.mp3" {
		t.Errorf("ожидали path-style /test-bucket/tracks/o/1.mp3, получили %q", parsed.Path)
	}
	q := parsed.Query()
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" {
		t.Errorf("ожидали алгоритм SigV4, получили %q", q.Get("X-Amz-Algorithm"))
	}
	if q.Get("X-Amz-Signature") == "" {
		t.Error("presigned URL должен содержать X-Amz-Signature")
	}
	if q.Get("X-Amz-Expires") != "86400" {
		t.Errorf("X-Amz-Expires должен соответствовать TTL 24h (86400), получили %q", q.Get("X-Amz-Expires"))
	}
	if !strings.HasPrefix(q.Get("X-Amz-Credential"), "AKIA/") {
		t.Errorf("X-Amz-Credential должен начинаться с accessKey, получили %q", q.Get("X-Amz-Credential"))
	}
}

func TestResolvePlayURL(t *testing.T) {
	stored := "https://cdn.example.com/tracks/o/1.mp3"
	c := New("https://s3.example.com", "us-east-1", "test-bucket", "AKIA", "secret").
		WithPublicBaseURL("https://cdn.example.com")

	// presign выключен → URL возвращается как есть (обратная совместимость).
	got, err := c.ResolvePlayURL(context.Background(), stored, time.Hour)
	if err != nil {
		t.Fatalf("ResolvePlayURL (disabled): %v", err)
	}
	if got != stored {
		t.Errorf("при выключенном presign ожидали URL как есть, получили %q", got)
	}

	// presign включён → подписанная ссылка, ключ извлечён из CDN-базы.
	c.WithPresign(true)
	got, err = c.ResolvePlayURL(context.Background(), stored, time.Hour)
	if err != nil {
		t.Fatalf("ResolvePlayURL (enabled): %v", err)
	}
	if !strings.Contains(got, "X-Amz-Signature=") {
		t.Errorf("при включённом presign ожидали подписанную ссылку, получили %q", got)
	}
	if !strings.Contains(got, "/test-bucket/tracks/o/1.mp3") {
		t.Errorf("ключ должен извлечься из CDN-ссылки, получили %q", got)
	}

	// legacy public URL (прямой S3-origin, до подключения CDN) тоже подписывается.
	legacy := "https://s3.example.com/test-bucket/tracks/o/2.mp3"
	if got, err = c.ResolvePlayURL(context.Background(), legacy, time.Hour); err != nil || !strings.Contains(got, "X-Amz-Signature=") {
		t.Errorf("legacy S3-URL должен подписываться, получили %q (err %v)", got, err)
	}

	// чужой URL (не наш бакет) подписать нельзя → возвращаем как есть.
	foreign := "https://evil.example.com/x.mp3"
	if got, err = c.ResolvePlayURL(context.Background(), foreign, time.Hour); err != nil || got != foreign {
		t.Errorf("чужой URL должен возвращаться как есть, получили %q (err %v)", got, err)
	}
}
