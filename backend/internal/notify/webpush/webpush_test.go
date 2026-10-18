package webpush

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// browser は、購読するブラウザの役（鍵を持ち、届いた本文を復号する）
type browser struct {
	key  *ecdh.PrivateKey
	auth []byte
}

func newBrowser(t *testing.T) *browser {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	return &browser{key: key, auth: auth}
}

func (b *browser) subscription(endpoint string) Subscription {
	var s Subscription
	s.Endpoint = endpoint
	s.Keys.P256dh = base64.RawURLEncoding.EncodeToString(b.key.PublicKey().Bytes())
	s.Keys.Auth = base64.RawURLEncoding.EncodeToString(b.auth)
	return s
}

// decrypt は、RFC 8291 の受け手として本文を復号する
func (b *browser) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()

	salt := body[:16]
	rs := binary.BigEndian.Uint32(body[16:20])
	idlen := int(body[20])
	asPublicRaw := body[21 : 21+idlen]
	ciphertext := body[21+idlen:]

	if rs != recordSize || idlen != 65 {
		t.Fatalf("unexpected header rs=%d idlen=%d", rs, idlen)
	}

	asPublic, err := ecdh.P256().NewPublicKey(asPublicRaw)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := b.key.ECDH(asPublic)
	if err != nil {
		t.Fatal(err)
	}

	cek, nonce, err := deriveKeys(shared, b.auth, b.key.PublicKey().Bytes(), asPublicRaw, salt)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain[len(plain)-1] != 0x02 {
		t.Fatalf("missing last-record delimiter")
	}
	return plain[:len(plain)-1]
}

func testKeys(t *testing.T) *Keys {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := ParseKeys(base64.RawURLEncoding.EncodeToString(k.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	// 公開鍵は秘密鍵から計算したものと一致する
	if keys.Public != base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()) {
		t.Fatal("public key mismatch")
	}
	return keys
}

func TestSend(t *testing.T) {

	keys := testKeys(t)
	b := newBrowser(t)
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	var got struct {
		header http.Header
		body   []byte
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.header = r.Header.Clone()
		got.body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	s := NewSender(keys, "https://example.com/contact")
	s.now = func() time.Time { return now }

	payload := []byte(`{"title":"三田線: 運転見合わせ","body":"人身事故の影響で…","url":"/routes/odpt.Railway%3AToei.Mita"}`)
	if err := s.Send(context.Background(), b.subscription(server.URL+"/push/abc"), payload, time.Hour); err != nil {
		t.Fatal(err)
	}

	if string(b.decrypt(t, got.body)) != string(payload) {
		t.Fatalf("payload mismatch")
	}
	if got.header.Get("Content-Encoding") != "aes128gcm" || got.header.Get("TTL") != "3600" {
		t.Fatalf("unexpected headers %v", got.header)
	}

	// Authorization: vapid t=<JWT>, k=<公開鍵>
	auth := got.header.Get("Authorization")
	jwt, k, ok := strings.Cut(strings.TrimPrefix(auth, "vapid t="), ", k=")
	if !ok || k != keys.Public {
		t.Fatalf("unexpected authorization %q", auth)
	}

	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("malformed jwt")
	}
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if err := json.Unmarshal(raw, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Aud != server.URL || claims.Sub != "https://example.com/contact" || claims.Exp != now.Add(jwtTTL).Unix() {
		t.Fatalf("unexpected claims %+v", claims)
	}

	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, sv := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if len(sig) != 64 || !ecdsa.Verify(&keys.private.PublicKey, digest[:], r, sv) {
		t.Fatal("bad vapid signature")
	}
}

func TestSendGone(t *testing.T) {

	for _, code := range []int{http.StatusNotFound, http.StatusGone} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		err := NewSender(testKeys(t), "mailto:a@example.com").Send(context.Background(), newBrowser(t).subscription(server.URL), []byte("x"), time.Minute)
		server.Close()
		if !errors.Is(err, ErrGone) {
			t.Errorf("%d: want ErrGone, got %v", code, err)
		}
	}
}

func TestSendErrorHidesEndpoint(t *testing.T) {

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	err := NewSender(testKeys(t), "mailto:a@example.com").Send(context.Background(), newBrowser(t).subscription(server.URL+"/secret-token"), []byte("x"), time.Minute)
	if err == nil || errors.Is(err, ErrGone) || strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestInvalid(t *testing.T) {

	if _, err := ParseKeys("not-a-key"); !errors.Is(err, ErrInvalidKey) {
		t.Errorf("want ErrInvalidKey, got %v", err)
	}

	sub := newBrowser(t).subscription("https://example.com")
	sub.Keys.P256dh = "AAAA"
	if _, err := encrypt(sub, []byte("x")); err == nil {
		t.Error("invalid p256dh should fail")
	}

	big := make([]byte, recordSize)
	if _, err := encrypt(newBrowser(t).subscription("https://example.com"), big); err == nil {
		t.Error("too large payload should fail")
	}
}

// RFC 8291 Appendix A の例を、ブラウザの鍵で復号できること（鍵の導出が仕様どおりか）
func TestRFC8291Example(t *testing.T) {

	d := func(s string) []byte {
		b, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	uaPrivate, err := ecdh.P256().NewPrivateKey(d("q1dXpw3UpT5VOmu_cf_v6ih07Aems3njxI-JWgLcM94"))
	if err != nil {
		t.Fatal(err)
	}
	b := &browser{key: uaPrivate, auth: d("BTBZMqHH6r4Tts7J_aSIgg")}

	if got := base64.RawURLEncoding.EncodeToString(uaPrivate.PublicKey().Bytes()); got != "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4" {
		t.Fatalf("ua_public mismatch: %s", got)
	}

	body := d("DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN")

	if got := string(b.decrypt(t, body)); got != "When I grow up, I want to be a watermelon" {
		t.Fatalf("unexpected plaintext %q", got)
	}
}

// push サービスのリダイレクトには従わない（許可したホストの外に送らない）
func TestSendDoesNotFollowRedirect(t *testing.T) {

	hit := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer other.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()

	err := NewSender(testKeys(t), "mailto:a@example.com").Send(context.Background(), newBrowser(t).subscription(server.URL), []byte("x"), time.Minute)
	if err == nil || hit {
		t.Fatalf("redirect should not be followed: err=%v hit=%v", err, hit)
	}
}
