package robokassa

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/numaestra/numaestra/pkg/circuitbreaker"
)

func refundSuccessHandler(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.Contains(r.URL.Path, "OpStateExt"):
		fmt.Fprint(w, `<?xml version="1.0"?><OperationStateResponse xmlns="http://merchant.roboxchange.com/WebService/"><Result><Code>0</Code></Result><Info><OpKey>op-key</OpKey></Info></OperationStateResponse>`)
	case strings.HasSuffix(r.URL.Path, "/Refund/Create"):
		fmt.Fprint(w, `{"success":true,"requestId":"req-1"}`)
	case strings.HasSuffix(r.URL.Path, "/Refund/GetState"):
		fmt.Fprint(w, `{"requestId":"req-1","amount":1500,"label":"finished"}`)
	}
}

func TestRefunderWithBreaker_PassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(refundSuccessHandler))
	defer srv.Close()

	c := newTestRefundClient(srv)
	r := NewRefunderWithBreaker(c)
	if err := r.Refund(context.Background(), "1500.00", 42); err != nil {
		t.Fatalf("ожидали успех, получили: %v", err)
	}
}

func TestRefunderWithBreaker_PropagatesInnerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "OpStateExt") {
			refundSuccessHandler(w, r)
			return
		}
		fmt.Fprint(w, `{"success":false,"message":"AlreadyRefunded"}`)
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestRefundClient(srv)
	r := &RefunderWithBreaker{
		inner:   c,
		breaker: circuitbreaker.New("test-robokassa", 3, time.Minute),
	}

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
	srv := httptest.NewServer(http.HandlerFunc(refundSuccessHandler))
	defer srv.Close()

	c := newTestRefundClient(srv)
	r := NewRefunderWithBreaker(c)
	var _ interface {
		Refund(ctx context.Context, outSum string, invID int64) error
	} = r
}
