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
