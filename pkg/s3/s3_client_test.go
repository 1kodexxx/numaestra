package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadFromURL_Success(t *testing.T) {
	const audio = "fake-mp3-bytes"
	var putBody string
	var gotAuth, gotDate, gotSHA string

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
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	c := New(srv.URL, "us-east-1", "test-bucket", "AKIA", "secret")
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
	if _, err := c.UploadFromURL(context.Background(), srv.URL+"/a.mp3", "k.mp3", "audio/mpeg"); err == nil {
		t.Fatal("ожидали ошибку при отказе S3 (403)")
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
