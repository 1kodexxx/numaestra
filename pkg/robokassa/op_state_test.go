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

// GetPaidAmountKopecks должен возвращать сумму строковым разбором, без float:
// нецелое число рублей (1999.99) обязано давать ровно 199999 копеек, иначе
// сверка с суммой заказа в sync-payment ложно бы не совпала.
func TestGetPaidAmountKopecks_NonRoundAmountNoFloatError(t *testing.T) {
	cases := []struct {
		outSum string
		want   int64
	}{
		{"2000.00", 200000},
		{"1999.99", 199999},
		{"1340.10", 134010},
		{"0.01", 1},
		{"1800", 180000},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintf(w, `<?xml version="1.0"?><OperationStateResponse xmlns="http://merchant.roboxchange.com/WebService/"><Result><Code>0</Code></Result><State><Code>100</Code></State><Info><OutSum>%s</OutSum><OpKey>k</OpKey></Info></OperationStateResponse>`, tc.outSum)
		}))

		c := New("shop", "p1", "p2", "p3", false)
		c.httpClient = srv.Client()
		c.opStateURL = srv.URL

		kopecks, paid, err := c.GetPaidAmountKopecks(context.Background(), 7)
		srv.Close()
		if err != nil {
			t.Fatalf("OutSum %q: неожиданная ошибка: %v", tc.outSum, err)
		}
		if !paid {
			t.Fatalf("OutSum %q: ожидали paid=true", tc.outSum)
		}
		if kopecks != tc.want {
			t.Fatalf("OutSum %q: ожидали %d копеек, получили %d", tc.outSum, tc.want, kopecks)
		}
	}
}
