package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const testClientID = "test-client.apps.googleusercontent.com"

type testKeys struct {
	keys    map[string]*rsa.PrivateKey
	fetches atomic.Int32
	server  *httptest.Server
}

func newTestKeys(t *testing.T, kids ...string) *testKeys {
	t.Helper()

	tk := &testKeys{keys: map[string]*rsa.PrivateKey{}}
	for _, kid := range kids {
		tk.add(t, kid)
	}

	tk.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tk.fetches.Add(1)

		type jwk struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		}
		var set struct {
			Keys []jwk `json:"keys"`
		}
		for kid, k := range tk.keys {
			set.Keys = append(set.Keys, jwk{
				Kty: "RSA",
				Kid: kid,
				N:   base64.RawURLEncoding.EncodeToString(k.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.E)).Bytes()),
			})
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(tk.server.Close)

	return tk
}

func (tk *testKeys) add(t *testing.T, kid string) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tk.keys[kid] = k
}

func (tk *testKeys) verifier(now time.Time) *GoogleVerifier {
	v := NewGoogleVerifier(testClientID)
	v.certsURL = tk.server.URL
	v.now = func() time.Time { return now }
	return v
}

func segment(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func sign(t *testing.T, key *rsa.PrivateKey, header, claims any) string {
	t.Helper()
	signing := segment(t, header) + "." + segment(t, claims)
	digest := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func validClaims(now time.Time) map[string]any {
	return map[string]any{
		"iss": "https://accounts.google.com",
		"aud": testClientID,
		"sub": "1234567890",
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
}

func TestGoogleVerifier(t *testing.T) {

	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	tk := newTestKeys(t, "k1")
	header := map[string]string{"alg": "RS256", "kid": "k1", "typ": "JWT"}

	with := func(k string, v any) map[string]any {
		c := validClaims(now)
		c[k] = v
		return c
	}

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
		ok    bool
	}{
		{"valid", sign(t, tk.keys["k1"], header, validClaims(now)), true},
		{"issuer without scheme", sign(t, tk.keys["k1"], header, with("iss", "accounts.google.com")), true},
		{"expired within skew", sign(t, tk.keys["k1"], header, with("exp", now.Add(-30*time.Second).Unix())), true},
		{"expired", sign(t, tk.keys["k1"], header, with("exp", now.Add(-2*time.Minute).Unix())), false},
		{"issued in the future", sign(t, tk.keys["k1"], header, with("iat", now.Add(5*time.Minute).Unix())), false},
		{"other audience", sign(t, tk.keys["k1"], header, with("aud", "other.apps.googleusercontent.com")), false},
		{"other issuer", sign(t, tk.keys["k1"], header, with("iss", "https://evil.example.com")), false},
		{"empty subject", sign(t, tk.keys["k1"], header, with("sub", "")), false},
		{"signed with another key", sign(t, other, header, validClaims(now)), false},
		{"alg none", segment(t, map[string]string{"alg": "none", "kid": "k1"}) + "." + segment(t, validClaims(now)) + ".", false},
		{"unknown kid", sign(t, tk.keys["k1"], map[string]string{"alg": "RS256", "kid": "nope"}, validClaims(now)), false},
		{"malformed", "abc.def", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, err := tk.verifier(now).Verify(context.Background(), tt.token)
			if tt.ok {
				if err != nil || sub != "1234567890" {
					t.Fatalf("want ok, got %q %v", sub, err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("want ErrInvalidToken, got %q %v", sub, err)
			}
		})
	}
}

func TestGoogleVerifierKeyCache(t *testing.T) {

	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	tk := newTestKeys(t, "k1")
	v := tk.verifier(now)
	header := map[string]string{"alg": "RS256", "kid": "k1"}

	for range 3 {
		if _, err := v.Verify(context.Background(), sign(t, tk.keys["k1"], header, validClaims(now))); err != nil {
			t.Fatal(err)
		}
	}
	if got := tk.fetches.Load(); got != 1 {
		t.Fatalf("keys should be cached: fetched %d times", got)
	}

	// 知らない kid は、直前に取ったばかりなら取り直さない
	tk.add(t, "k2")
	header2 := map[string]string{"alg": "RS256", "kid": "k2"}
	if _, err := v.Verify(context.Background(), sign(t, tk.keys["k2"], header2, validClaims(now))); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
	if got := tk.fetches.Load(); got != 1 {
		t.Fatalf("should not refetch within a minute: fetched %d times", got)
	}

	// 時間がたてば、鍵の入れ替えに追いつく
	now = now.Add(2 * time.Minute)
	v.now = func() time.Time { return now }
	if _, err := v.Verify(context.Background(), sign(t, tk.keys["k2"], header2, validClaims(now))); err != nil {
		t.Fatalf("new key should be fetched: %v", err)
	}

	// 期限（max-age）が切れたら取り直す
	now = now.Add(2 * time.Hour)
	before := tk.fetches.Load()
	if _, err := v.Verify(context.Background(), sign(t, tk.keys["k1"], header, validClaims(now))); err != nil {
		t.Fatal(err)
	}
	if got := tk.fetches.Load(); got != before+1 {
		t.Fatalf("expired keys should be refetched: fetched %d times", got-before)
	}
}

func TestMaxAge(t *testing.T) {
	tests := map[string]time.Duration{
		"public, max-age=19800, must-revalidate": 19800 * time.Second,
		"no-cache":                               defaultKeysTTL,
		"":                                       defaultKeysTTL,
		"max-age=abc":                            defaultKeysTTL,
	}
	for in, want := range tests {
		if got := maxAge(in); got != want {
			t.Errorf("maxAge(%q) = %v, want %v", in, got, want)
		}
	}
}
