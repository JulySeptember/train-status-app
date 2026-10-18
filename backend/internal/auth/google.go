// Package auth は、Google アカウントでのログインとセッションを扱う（docs/design/login.md 3章）。
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GoogleCertsURL は、Google が ID トークンの署名に使う公開鍵（JWK）の一覧
const GoogleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

const (
	// 時計のずれとして許す幅
	clockSkew = time.Minute

	// Cache-Control に max-age が無いときに、鍵を使い回す時間
	defaultKeysTTL = time.Hour

	// 知らない kid のトークンが来たときに、鍵を取り直す間隔の下限（でたらめな kid で Google に問い合わせさせないため）
	minRefreshInterval = time.Minute
)

// googleIssuers は、Google の ID トークンの iss として正しい値
var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

// Verifier は、Google の ID トークンを確かめて、利用者 ID（sub）を返す。
type Verifier interface {
	Verify(ctx context.Context, idToken string) (subject string, err error)
}

// GoogleVerifier は、Google の公開鍵で ID トークン（RS256 の JWT）を確かめる。
// google.golang.org/api/idtoken は依存が大きく、実行ファイル（Lambda の起動時間）に響くので、標準ライブラリで確かめる。
type GoogleVerifier struct {
	clientID string
	certsURL string
	client   *http.Client
	now      func() time.Time

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	fetchedAt time.Time
}

func NewGoogleVerifier(clientID string) *GoogleVerifier {
	return &GoogleVerifier{
		clientID: clientID,
		certsURL: GoogleCertsURL,
		client:   &http.Client{Timeout: 5 * time.Second},
		now:      time.Now,
	}
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type googleClaims struct {
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
}

func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (string, error) {

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("%w: malformed token", ErrInvalidToken)
	}

	var header jwtHeader
	if err := decodeSegment(parts[0], &header); err != nil {
		return "", fmt.Errorf("%w: header: %v", ErrInvalidToken, err)
	}
	// alg を見て検証の方法を変えない（none や HS256 にすり替えられないように）
	if header.Alg != "RS256" {
		return "", fmt.Errorf("%w: unsupported alg %q", ErrInvalidToken, header.Alg)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("%w: signature: %v", ErrInvalidToken, err)
	}

	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return "", err
	}

	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return "", fmt.Errorf("%w: bad signature", ErrInvalidToken)
	}

	var claims googleClaims
	if err := decodeSegment(parts[1], &claims); err != nil {
		return "", fmt.Errorf("%w: claims: %v", ErrInvalidToken, err)
	}

	now := v.now()
	switch {
	case !slices.Contains(googleIssuers, claims.Iss):
		return "", fmt.Errorf("%w: unexpected issuer", ErrInvalidToken)
	case claims.Aud != v.clientID:
		return "", fmt.Errorf("%w: unexpected audience", ErrInvalidToken)
	case claims.Sub == "":
		return "", fmt.Errorf("%w: empty subject", ErrInvalidToken)
	case now.After(time.Unix(claims.Exp, 0).Add(clockSkew)):
		return "", fmt.Errorf("%w: expired", ErrInvalidToken)
	case time.Unix(claims.Iat, 0).After(now.Add(clockSkew)):
		return "", fmt.Errorf("%w: issued in the future", ErrInvalidToken)
	}

	return claims.Sub, nil
}

// key は kid の公開鍵を返す。期限が切れているか、知らない kid なら取り直す
func (v *GoogleVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {

	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now()
	key, ok := v.keys[kid]
	if ok && now.Before(v.expiresAt) {
		return key, nil
	}

	// 鍵が入れ替わったばかりのことがあるので、知らない kid でも取り直す。ただし間隔をあける
	if !ok && v.keys != nil && now.Before(v.expiresAt) && now.Sub(v.fetchedAt) < minRefreshInterval {
		return nil, fmt.Errorf("%w: unknown key id", ErrInvalidToken)
	}

	if err := v.fetchKeys(ctx); err != nil {
		return nil, err
	}

	if key, ok := v.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("%w: unknown key id", ErrInvalidToken)
}

type jwkSet struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *GoogleVerifier) fetchKeys(ctx context.Context) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.certsURL, nil)
	if err != nil {
		return err
	}

	res, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch google certs: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch google certs: status %d", res.StatusCode)
	}

	var set jwkSet
	if err := json.NewDecoder(res.Body).Decode(&set); err != nil {
		return fmt.Errorf("decode google certs: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		n, errN := base64.RawURLEncoding.DecodeString(k.N)
		e, errE := base64.RawURLEncoding.DecodeString(k.E)
		if errN != nil || errE != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		}
	}
	if len(keys) == 0 {
		return errors.New("google certs: no usable keys")
	}

	now := v.now()
	v.keys = keys
	v.fetchedAt = now
	v.expiresAt = now.Add(maxAge(res.Header.Get("Cache-Control")))

	return nil
}

// maxAge は Cache-Control の max-age を返す。無ければ defaultKeysTTL
func maxAge(cacheControl string) time.Duration {
	for _, directive := range strings.Split(cacheControl, ",") {
		value, ok := strings.CutPrefix(strings.TrimSpace(directive), "max-age=")
		if !ok {
			continue
		}
		if sec, err := strconv.Atoi(value); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return defaultKeysTTL
}

func decodeSegment(segment string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
