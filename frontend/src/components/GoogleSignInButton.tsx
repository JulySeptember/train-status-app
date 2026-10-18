import { useEffect, useRef, useState } from "react";

import { loadGoogleIdentity } from "@/lib/auth";

// 「Google でログイン」のボタン。ボタンそのものは Google が描く（iframe）
export default function GoogleSignInButton({
  clientId,
  onCredential,
}: {
  clientId: string;
  onCredential: (credential: string) => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [failed, setFailed] = useState(false);

  // initialize に渡した関数は後から差し替えられないので、最新の onCredential を ref で参照する
  const callback = useRef(onCredential);
  useEffect(() => {
    callback.current = onCredential;
  }, [onCredential]);

  useEffect(() => {
    let cancelled = false;

    loadGoogleIdentity()
      .then((id) => {
        if (cancelled || !ref.current) {
          return;
        }
        id.initialize({
          client_id: clientId,
          callback: (res) => callback.current(res.credential),
          ux_mode: "popup",
        });
        id.renderButton(ref.current, {
          theme: "outline",
          size: "large",
          text: "signin_with",
          shape: "pill",
          locale: "ja",
        });
      })
      .catch(() => {
        if (!cancelled) {
          setFailed(true);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [clientId]);

  if (failed) {
    return (
      <p className="text-sm text-destructive-foreground">
        Google
        のログインを読み込めませんでした。広告ブロックなどの拡張機能を確認して、ページを開き直してください。
      </p>
    );
  }

  return <div ref={ref} className="min-h-10" />;
}
