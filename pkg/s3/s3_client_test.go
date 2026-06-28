package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
