// 運行情報の通知を受け取る Service Worker（docs/design/login.md 5.4）。
// 通知を出すだけで、ページのキャッシュ（オフライン対応）はしない。
// バックエンドの notify.Message（title・body・url・tag）を受け取る

self.addEventListener("push", (event) => {
  let message = {};
  try {
    message = event.data ? event.data.json() : {};
  } catch {
    // 形の違う通知は、題名だけ出す
  }

  event.waitUntil(
    self.registration.showNotification(message.title || "NORIKAE AI", {
      body: message.body || "",
      tag: message.tag,
      // 同じ路線の新しい通知で置き換えたときも、知らせる
      renotify: Boolean(message.tag),
      icon: "/icon-192.png",
      // Android のステータスバーに出す白一色のアイコン
      badge: "/badge-96.png",
      data: { url: message.url || "/" },
    }),
  );
});

// ブラウザが購読を作り直した・期限切れにしたときに、新しい購読をサーバーに登録し直す
// （しないと、サーバーは古い endpoint に送り続けて、通知が届かなくなる）。
// 同じオリジンへの fetch なので、ログインの Cookie が付き、CSRF 対策（Sec-Fetch-Site）も通る。
// ログアウト・退会したときはブラウザの購読を外しているので、前の利用者の購読を作り直すことは無い
self.addEventListener("pushsubscriptionchange", (event) => {
  event.waitUntil(resubscribe(event.oldSubscription, event.newSubscription));
});

async function resubscribe(oldSubscription, newSubscription) {
  let subscription = newSubscription;
  if (!subscription) {
    // 前と同じ鍵で購読し直す。前の購読の鍵が分からなければ、今の公開鍵を取る
    let key = oldSubscription?.options?.applicationServerKey;
    if (!key) {
      const res = await fetch("/api/push/config");
      if (!res.ok) {
        return; // 通知を使えない（鍵が無い）
      }
      key = (await res.json()).publicKey;
    }
    subscription = await self.registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: key,
    });
  }

  await fetch("/api/me/push-subscriptions", {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(subscription.toJSON()),
  });

  // 古い購読はサーバーから外す（失敗しても、次に送ったときに 410 が返って消える）
  if (
    oldSubscription?.endpoint &&
    oldSubscription.endpoint !== subscription.endpoint
  ) {
    await fetch("/api/me/push-subscriptions", {
      method: "DELETE",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ endpoint: oldSubscription.endpoint }),
    }).catch(() => {});
  }
}

// 通知を押したら、開いている画面があればそこで、無ければ新しく開く
self.addEventListener("notificationclick", (event) => {
  event.notification.close();

  // 自分のサイトの中だけを開く
  const url = new URL(
    event.notification.data?.url || "/",
    self.location.origin,
  );
  if (url.origin !== self.location.origin) {
    return;
  }

  event.waitUntil(
    self.clients
      .matchAll({ type: "window", includeUncontrolled: true })
      .then((clients) => {
        const client = clients.find((c) => "focus" in c);
        if (client) {
          // まだこの Service Worker の管理下にないタブは navigate できないので、新しく開く
          return client
            .navigate(url.href)
            .then((c) => (c ?? client).focus())
            .catch(() => self.clients.openWindow(url.href));
        }
        return self.clients.openWindow(url.href);
      }),
  );
});
