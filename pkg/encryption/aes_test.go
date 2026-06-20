package encryption

import (
	"strings"
	"testing"
)

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c, err := New(testKey())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	want := "session_cookie_value_12345"
	enc, err := c.Encrypt(want)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == want {
		t.Fatal("шифртекст совпадает с открытым текстом")
	}

	got, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != want {
		t.Errorf("ожидали %q, получили %q", want, got)
	}
}

func TestEncryptDecrypt_EmptyString(t *testing.T) {
	c, _ := New(testKey())

	enc, err := c.Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	got, err := c.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if got != "" {
		t.Errorf("ожидали пустую строку, получили %q", got)
	}
}

func TestEncryptDecrypt_UniqueNoncePerCall(t *testing.T) {
	c, _ := New(testKey())

	enc1, _ := c.Encrypt("same")
	enc2, _ := c.Encrypt("same")
	if enc1 == enc2 {
		t.Error("два шифрования одного текста дали одинаковый результат — nonce не случаен?")
	}
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 0xFF

	c1, _ := New(key1)
	c2, _ := New(key2)

	enc, _ := c1.Encrypt("секрет")
	_, err := c2.Decrypt(enc)
	if err == nil {
		t.Fatal("расшифровка другим ключом должна вернуть ошибку")
	}
}

func TestEncryptDecrypt_TamperedCiphertext(t *testing.T) {
	c, _ := New(testKey())

	enc, _ := c.Encrypt("данные")
	// Заменяем последние 4 символа base64 — декодирование пройдёт,
	// но тег GCM не совпадёт.
	tampered := enc[:len(enc)-4] + "AAAA"
	_, err := c.Decrypt(tampered)
	if err == nil {
		t.Fatal("расшифровка изменённого шифртекста должна вернуть ошибку")
	}
}

func TestNew_InvalidKeyLength(t *testing.T) {
	_, err := New(make([]byte, 16))
	if err == nil {
		t.Fatal("ожидали ошибку для 16-байтового ключа (AES-128 не принимаем)")
	}
	if !strings.Contains(err.Error(), "32") {
		t.Errorf("сообщение об ошибке должно содержать '32', получили: %v", err)
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	c, _ := New(testKey())

	_, err := c.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Fatal("ожидали ошибку для невалидного base64")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	c, _ := New(testKey())

	// base64("X") — слишком короткий, чтобы содержать nonce
	_, err := c.Decrypt("WA==")
	if err == nil {
		t.Fatal("ожидали ошибку для слишком короткого шифртекста")
	}
}
