import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "@/api";
import { useMe } from "@/lib/auth";
import {
  forgetPushOwner,
  pushOwnerGeneration,
  rememberPushOwner,
} from "@/lib/pushOwner";

const DEVICE_KEY = ["pushDevice"];

// この端末（ブラウザ）が Web Push に対応しているか
export function pushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

// iPhone・iPad は、ホーム画面に追加して開いたときだけ通知を受け取れる（iOS 16.4 以降）
export function needsHomeScreen(): boolean {
  const ios =
    /iPhone|iPad|iPod/.test(navigator.userAgent) ||
    (navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1);
  const standalone =
    window.matchMedia("(display-mode: standalone)").matches ||
    ("standalone" in navigator &&
      (navigator as { standalone?: boolean }).standalone === true);
  return ios && !standalone;
}

// base64url（VAPID の公開鍵）を applicationServerKey に渡す形にする
function decodeKey(base64url: string): Uint8Array<ArrayBuffer> {
  const base64 = base64url.replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64 + "=".repeat((4 - (base64.length % 4)) % 4));
  const bytes = new Uint8Array(new ArrayBuffer(raw.length));
  for (let i = 0; i < raw.length; i++) {
    bytes[i] = raw.charCodeAt(i);
  }
  return bytes;
}

// この端末の購読。無ければ null
async function currentSubscription(): Promise<PushSubscription | null> {
  if (!pushSupported()) {
    return null;
  }
  const registration = await navigator.serviceWorker.getRegistration("/");
  return (await registration?.pushManager.getSubscription()) ?? null;
}

export function usePushConfig() {
  return useQuery({
    queryKey: ["pushConfig"],
    queryFn: api.getPushConfig,
    staleTime: Infinity,
  });
}

// 購読が今の VAPID の公開鍵で作られたか（鍵を作り直すと、古い購読には届かない）
function sameKey(subscription: PushSubscription, publicKey: string): boolean {
  const key = subscription.options.applicationServerKey;
  if (!key) {
    return false;
  }
  const a = new Uint8Array(key);
  const b = decodeKey(publicKey);
  return a.length === b.length && a.every((v, i) => v === b[i]);
}

// この端末で、ログイン中の利用者が通知を受け取る設定になっているか。
// ブラウザに購読があっても、サーバーでこの利用者の購読になっていなければ false（共用の端末で、前の人の購読を引き継がない）
export function usePushDevice() {
  const me = useMe();
  const config = usePushConfig();

  return useQuery({
    queryKey: DEVICE_KEY,
    queryFn: async () => {
      const subscription = await currentSubscription();
      if (
        !subscription ||
        Notification.permission !== "granted" ||
        !config.data ||
        !sameKey(subscription, config.data.publicKey)
      ) {
        return false;
      }
      const since = pushOwnerGeneration();
      const status = await api.subscriptionStatus(subscription.endpoint);
      // 前からの購読も、Service Worker が登録し直せるように記録する
      if (status.registered) {
        await rememberPushOwner(subscription.endpoint, since);
      }
      return status.registered;
    },
    enabled: !!me.data && pushSupported() && config.isSuccess,
    staleTime: Infinity,
    retry: false,
  });
}

// この端末で通知を受け取る
export function useEnablePush() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      const config = await api.getPushConfig();
      if (!config) {
        throw new Error("いまは通知を使えません。");
      }

      const permission = await Notification.requestPermission();
      if (permission !== "granted") {
        throw new Error(
          "通知が許可されていません。ブラウザの設定で、このサイトの通知を許可してください。",
        );
      }

      const registration = await navigator.serviceWorker.register("/sw.js");
      await navigator.serviceWorker.ready;

      let subscription = await registration.pushManager.getSubscription();
      // 鍵を作り直したあとの古い購読は、作り直す
      if (subscription && !sameKey(subscription, config.publicKey)) {
        await subscription.unsubscribe();
        subscription = null;
      }
      subscription ??= await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: decodeKey(config.publicKey),
      });

      const since = pushOwnerGeneration();
      await api.subscribePush(subscription.toJSON());
      await rememberPushOwner(subscription.endpoint, since);
    },
    onSuccess: () => client.setQueryData(DEVICE_KEY, true),
  });
}

// この端末で通知を受け取るのをやめる
export function useDisablePush() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      await forgetPushOwner();
      const subscription = await currentSubscription();
      if (subscription) {
        await api.unsubscribePush(subscription.endpoint);
        await subscription.unsubscribe();
      }
    },
    onSuccess: () => client.setQueryData(DEVICE_KEY, false),
  });
}

export function useAlerts() {
  const me = useMe();

  return useQuery({
    queryKey: ["alerts"],
    queryFn: api.getAlerts,
    enabled: !!me.data,
  });
}
