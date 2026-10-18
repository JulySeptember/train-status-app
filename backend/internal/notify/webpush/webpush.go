// Package webpush は、Web Push の通知を送る（docs/design/login.md 5.4）。
// 本文の暗号化（RFC 8291、aes128gcm）と VAPID（RFC 8292）を標準ライブラリで行う。
package webpush

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

var (
	// ErrGone は、購読が無くなっていること（404・410）。購読を消す
	ErrGone = errors.New("webpush: subscription is gone")

	// ErrInvalidKey は、VAPID の秘密鍵が正しくないこと
	ErrInvalidKey = errors.New("webpush: invalid vapid key")
)

const (
	// 暗号化したレコードの大きさ（RFC 8188）。1レコードに収まる本文だけを送る
	recordSize = 4096

	// VAPID の JWT の有効期限（24時間まで）
	jwtTTL = 12 * time.Hour
)

// Subscription は、ブラウザの PushSubscription（toJSON した形）
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Keys は VAPID の鍵の組
type Keys struct {
	private *ecdsa.PrivateKey

	// 公開鍵（非圧縮の点、base64url）。ブラウザの applicationServerKey に渡す
	Public string
}

// ParseKeys は、base64url（パディングなし）の P-256 の秘密鍵から鍵の組を作る（cmd/vapid-key の出力）
func ParseKeys(privateKey string) (*Keys, error) {

	raw, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	priv, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	pub, err := priv.PublicKey.Bytes()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKey, err)
	}

	return &Keys{private: priv, Public: base64.RawURLEncoding.EncodeToString(pub)}, nil
}

type Sender struct {
	keys    *Keys
	subject string
	client  *http.Client
	now     func() time.Time
}

// NewSender は送り手を作る。subject は push サービスが連絡に使う URL か mailto:
func NewSender(keys *Keys, subject string) *Sender {
	return &Sender{
		keys:    keys,
		subject: subject,
		client: &http.Client{
			Timeout: 10 * time.Second,
			// push サービスのリダイレクトには従わない（許可したホストの外に送らない）
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		now: time.Now,
	}
}

// Send は、payload を暗号化して購読の endpoint に送る。ttl は push サービスが届けるのを待つ時間。
// 購読が無くなっていれば ErrGone を返す。エラーに endpoint は含めない（利用者を特定できるため）
func (s *Sender) Send(ctx context.Context, sub Subscription, payload []byte, ttl time.Duration) error {

	body, err := encrypt(sub, payload)
	if err != nil {
		return err
	}

	endpoint, err := url.Parse(sub.Endpoint)
	if err != nil {
		return fmt.Errorf("webpush: invalid endpoint")
	}

	jwt, err := s.vapidJWT(endpoint.Scheme + "://" + endpoint.Host)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webpush: invalid endpoint")
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	req.Header.Set("Urgency", "high")
	req.Header.Set("Authorization", "vapid t="+jwt+", k="+s.keys.Public)

	res, err := s.client.Do(req)
	if err != nil {
		// *url.Error は endpoint を含むので、そのまま返さない
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return fmt.Errorf("webpush: send to %s: %v", endpoint.Host, urlErr.Err)
		}
		return fmt.Errorf("webpush: send to %s: %v", endpoint.Host, err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))

	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return ErrGone
	case res.StatusCode >= 300:
		return fmt.Errorf("webpush: %s returned %d", endpoint.Host, res.StatusCode)
	}
	return nil
}

// vapidJWT は、push サービス（audience）向けの ES256 の JWT を作る
func (s *Sender) vapidJWT(audience string) (string, error) {

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`))
	claims, err := json.Marshal(map[string]any{
		"aud": audience,
		"exp": s.now().Add(jwtTTL).Unix(),
		"sub": s.subject,
	})
	if err != nil {
		return "", err
	}

	signing := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))

	r, sig, err := ecdsa.Sign(rand.Reader, s.keys.private, digest[:])
	if err != nil {
		return "", err
	}

	// JWS の ES256 の署名は、r と s をそれぞれ 32 バイトにして並べる（ASN.1 ではない）
	raw := make([]byte, 64)
	r.FillBytes(raw[:32])
	sig.FillBytes(raw[32:])

	return signing + "." + base64.RawURLEncoding.EncodeToString(raw), nil
}

// encrypt は、RFC 8291 の aes128gcm で payload を暗号化する
func encrypt(sub Subscription, payload []byte) ([]byte, error) {

	uaPublicRaw, err := base64.RawURLEncoding.DecodeString(sub.Keys.P256dh)
	if err != nil {
		return nil, fmt.Errorf("webpush: invalid p256dh: %v", err)
	}
	authSecret, err := base64.RawURLEncoding.DecodeString(sub.Keys.Auth)
	if err != nil || len(authSecret) == 0 {
		return nil, fmt.Errorf("webpush: invalid auth secret")
	}

	uaPublic, err := ecdh.P256().NewPublicKey(uaPublicRaw)
	if err != nil {
		return nil, fmt.Errorf("webpush: invalid p256dh: %v", err)
	}

	// 送るたびに作る使い捨ての鍵（application server の鍵）
	asPrivate, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	asPublicRaw := asPrivate.PublicKey().Bytes()

	shared, err := asPrivate.ECDH(uaPublic)
	if err != nil {
		return nil, err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	cek, nonce, err := deriveKeys(shared, authSecret, uaPublicRaw, asPublicRaw, salt)
	if err != nil {
		return nil, err
	}

	// 本文のあとに、最後のレコードであることを示す 0x02 を付ける
	plaintext := append(append([]byte{}, payload...), 0x02)
	if len(plaintext)+16 > recordSize-86 {
		return nil, fmt.Errorf("webpush: payload too large (%d bytes)", len(payload))
	}

	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// ヘッダ: salt(16) | rs(4) | idlen(1) | keyid（使い捨ての公開鍵、65）
	header := make([]byte, 0, 86)
	header = append(header, salt...)
	header = binary.BigEndian.AppendUint32(header, recordSize)
	header = append(header, byte(len(asPublicRaw)))
	header = append(header, asPublicRaw...)

	return gcm.Seal(header, nonce, plaintext, nil), nil
}

// deriveKeys は、共有の秘密から本文の鍵（CEK）と nonce を作る（RFC 8291 3.4・RFC 8188 2.2）
func deriveKeys(shared, authSecret, uaPublic, asPublic, salt []byte) (cek, nonce []byte, err error) {

	keyInfo := append([]byte("WebPush: info\x00"), uaPublic...)
	keyInfo = append(keyInfo, asPublic...)

	prkKey, err := hkdf.Extract(sha256.New, shared, authSecret)
	if err != nil {
		return nil, nil, err
	}
	ikm, err := hkdf.Expand(sha256.New, prkKey, string(keyInfo), 32)
	if err != nil {
		return nil, nil, err
	}

	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, nil, err
	}
	cek, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, nil, err
	}
	nonce, err = hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, nil, err
	}
	return cek, nonce, nil
}
