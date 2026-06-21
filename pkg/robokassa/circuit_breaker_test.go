package robokassa

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

func TestRefunderWithBreaker_PassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "OK42")
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	r := NewRefunderWithBreaker(c)
	if err := r.Refund(context.Background(), "1500.00", 42); err != nil {
		t.Fatalf("ожидали успех, получили: %v", err)
	}
}

func TestRefunderWithBreaker_PropagatesInnerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ERROR99")
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	r := NewRefunderWithBreaker(c)
	if err := r.Refund(context.Background(), "1500.00", 99); err == nil {
		t.Fatal("ожидали ошибку от сервера")
	}
}

func TestRefunderWithBreaker_OpensAfterThreshold(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	// threshold=3, короткий timeout чтобы не мешать тесту.
	r := &RefunderWithBreaker{
		inner:   c,
		breaker: circuitbreaker.New("test-robokassa", 3, time.Minute),
	}

	// 3 ошибки → автомат размыкается.
	for i := 0; i < 3; i++ {
		_ = r.Refund(context.Background(), "1500.00", int64(i))
	}

	before := callCount
	_ = r.Refund(context.Background(), "1500.00", 999)
	if callCount != before {
		t.Errorf("после открытия автомата inner.Refund не должен вызываться (было %d вызовов, стало %d)", before, callCount)
	}
}

func TestRefunderWithBreaker_ImplementsRefunder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "OK1")
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	r := NewRefunderWithBreaker(c)
	// Compile-time: *RefunderWithBreaker должен вызывать Refund с той же сигнатурой.
	var _ interface {
		Refund(ctx context.Context, outSum string, invID int64) error
	} = r
}
