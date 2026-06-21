package adminsession

import (
	"testing"
	"time"
)

func testSecret() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestIssueVerify_Roundtrip(t *testing.T) {
	token := Issue(testSecret(), "admin", time.Hour)
	login, ok := Verify(testSecret(), token)
	if !ok {
		t.Fatal("ожидали валидный токен")
	}
	if login != "admin" {
		t.Errorf("ожидали login=admin, получили %q", login)
	}
}

func TestVerify_Expired(t *testing.T) {
	token := Issue(testSecret(), "admin", -time.Minute) // уже истёк
	_, ok := Verify(testSecret(), token)
	if ok {
		t.Error("ожидали отказ для истёкшего токена")
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	token := Issue(testSecret(), "admin", time.Hour)
	_, ok := Verify([]byte("другой-секрет-другой-секрет-12"), token)
	if ok {
		t.Error("ожидали отказ при неверном секрете")
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	token := Issue(testSecret(), "admin", time.Hour)
	tampered := "YWRtaW4y" + token[8:] // подменяем часть payload, подпись не пересчитана
	_, ok := Verify(testSecret(), tampered)
	if ok {
		t.Error("ожидали отказ для подменённого payload")
	}
}

func TestVerify_MalformedToken(t *testing.T) {
	cases := []string{"", "no-dots-at-all", "a.b", "a.b.c.d", "!!!.!!!.!!!"}
	for _, c := range cases {
		if _, ok := Verify(testSecret(), c); ok {
			t.Errorf("ожидали отказ для некорректного токена %q", c)
		}
	}
}
