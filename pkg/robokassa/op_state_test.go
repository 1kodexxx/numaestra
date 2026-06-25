package robokassa

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsInvoicePaid_Paid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><OperationStateResponse xmlns="http://merchant.roboxchange.com/WebService/"><Result><Code>0</Code></Result><State><Code>100</Code></State><Info><OpKey>k</OpKey></Info></OperationStateResponse>`)
	}))
	defer srv.Close()

	c := New("shop", "p1", "p2", "p3", false)
	c.httpClient = srv.Client()
	c.opStateURL = srv.URL

	paid, err := c.IsInvoicePaid(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if !paid {
		t.Fatal("ожидали paid=true")
	}
}

func TestIsInvoicePaid_Pending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><OperationStateResponse xmlns="http://merchant.roboxchange.com/WebService/"><Result><Code>0</Code></Result><State><Code>5</Code></State><Info><OpKey>k</OpKey></Info></OperationStateResponse>`)
	}))
	defer srv.Close()

	c := New("shop", "p1", "p2", "p3", false)
	c.httpClient = srv.Client()
	c.opStateURL = srv.URL

	paid, err := c.IsInvoicePaid(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		t.Fatal("ожидали paid=false")
	}
}

func TestIsInvoicePaid_TestModeSkipped(t *testing.T) {
	c := New("shop", "p1", "p2", "p3", true)
	paid, err := c.IsInvoicePaid(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if paid {
		t.Fatal("в тестовом режиме не должны считать оплату подтверждённой")
	}
}
