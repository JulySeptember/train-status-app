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
// 同じオリジンへの fetch なので、ログインの Cookie が付き、CSRF 対策（Sec-Fetch-Site）も通る
self.addEventListener("pushsubscriptionchange", (event) => {
  event.waitUntil(resubscribe(event.oldSubscription, event.newSubscription));
});

function postJSON(method, url, body) {
  return fetch(url, {
    method,
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

// base64url（VAPID の公開鍵）をバイト列にする（src/lib/push.ts の decodeKey と同じ）
function decodeKey(base64url) {
  const base64 = base64url.replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64 + "=".repeat((4 - (base64.length % 4)) % 4));
  return Uint8Array.from(raw, (c) => c.charCodeAt(0));
}

// この端末の購読が今ログインしている利用者のものだという記録（src/lib/pushOwner.ts と名前を合わせる）
const OWNER_CACHE = "push-owner";
const OWNER_KEY = "/push-owner";

async function readOwner() {
  try {
    const res = await (await caches.open(OWNER_CACHE)).match(OWNER_KEY);
    return res ? await res.json() : null;
  } catch {
    return null;
  }
}

async function writeOwner(endpoint) {
  try {
    const cache = await caches.open(OWNER_CACHE);
    await cache.put(OWNER_KEY, new Response(JSON.stringify({ endpoint })));
  } catch {
    // 何もしない
  }
}

// 古い購読が今ログインしている利用者のものか。
// ページが記録した持ち主（ログイン・ログアウト・退会・停止で消える）と一致すれば、サーバーが送信の 410 で
// 古い購読を消したあとでも登録し直せる。記録が無ければ、サーバーに今も登録されているかで確かめる。
// （セッション切れや別のアカウントでのログインのあとに、前の利用者の購読を今の利用者に移さない）
async function ownedByCurrentUser(oldEndpoint) {
  const owner = await readOwner();
  if (owner?.endpoint && (!oldEndpoint || owner.endpoint === oldEndpoint)) {
    return owner.endpoint;
  }
  if (!oldEndpoint) {
    return null;
  }
  const status = await postJSON("POST", "/api/me/push-subscriptions/status", {
    endpoint: oldEndpoint,
  });
  if (status.ok && (await status.json()).registered) {
    return oldEndpoint;
  }
  return null;
}

async function resubscribe(oldSubscription, newSubscription) {
  // ブラウザによっては oldSubscription が無いので、そのときは記録した endpoint を古い購読とみなす
  const oldEndpoint = await ownedByCurrentUser(oldSubscription?.endpoint);
  if (!oldEndpoint) {
    return;
  }

  let subscription = newSubscription;
  if (!subscription) {
    // 今の公開鍵で購読し直す（鍵を作り直していても届くように）。取れなければ前と同じ鍵を使う
    let key = oldSubscription?.options?.applicationServerKey;
    const config = await fetch("/api/push/config").catch(() => null);
    if (config?.ok) {
      key = decodeKey((await config.json()).publicKey);
    }
    if (!key) {
      return;
    }
    subscription = await self.registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: key,
    });
  }

  const res = await postJSON(
    "POST",
    "/api/me/push-subscriptions",
    subscription.toJSON(),
  );
  if (!res.ok) {
    return;
  }
  await writeOwner(subscription.endpoint);

  // 古い購読はサーバーから外す（失敗しても、次に送ったときに 410 が返って消える）
  if (oldEndpoint !== subscription.endpoint) {
    await postJSON("DELETE", "/api/me/push-subscriptions", {
      endpoint: oldEndpoint,
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
