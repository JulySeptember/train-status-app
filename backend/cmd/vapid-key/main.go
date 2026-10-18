// Command vapid-key は、Web Push（運行情報の通知）に使う VAPID の秘密鍵（P-256）を作り、
// base64url（パディングなし）で標準出力に書く。公開鍵はバックエンドが秘密鍵から計算するので出さない。
//
// 秘密鍵が画面に出ないよう、出力はそのまま SSM に渡す（docs/design/login.md 5.4）:
//
//	cd backend && aws ssm put-parameter --region ap-northeast-1 --type SecureString \
//	  --name /train-status-app/dev/vapid-private-key --value "$(go run ./cmd/vapid-key)"
//
// 鍵を作り直すと、登録済みの通知の購読はすべて使えなくなる。
package main

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
)

func main() {
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(base64.RawURLEncoding.EncodeToString(key.Bytes()))
}
