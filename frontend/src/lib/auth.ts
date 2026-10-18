import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "@/api";

const ME_KEY = ["me"];

// ログインし直し・ログアウト・退会したら、前の利用者のデータ（lib/favorites.ts・lib/push.ts）を残さない
function clearUserData(client: ReturnType<typeof useQueryClient>) {
  for (const key of ["favorites", "alerts", "pushDevice"]) {
    client.removeQueries({ queryKey: [key] });
  }
}

// ログアウト・退会のときに、この端末の通知の購読を外す（次にこの端末を使う人に、前の人の通知を出さない）。
// server が true なら、サーバーの購読からも外す（ログアウトの前。退会ではサーバーがまとめて消す）。
// 失敗してもログアウト・退会は続ける（サーバーの購読は、退会すれば消える）
async function forgetThisDevice(server: boolean) {
  try {
    if (!("serviceWorker" in navigator)) {
      return;
    }
    const registration = await navigator.serviceWorker.getRegistration("/");
    const subscription = await registration?.pushManager?.getSubscription();
    if (subscription) {
      if (server) {
        await api.unsubscribePush(subscription.endpoint);
      }
      await subscription.unsubscribe();
    }
  } catch {
    // 何もしない
  }
}

// ログインの設定。ログインを使えない（バックエンドに設定が無い）ときは data が null
export function useAuthConfig() {
  return useQuery({
    queryKey: ["authConfig"],
    queryFn: api.getAuthConfig,
    staleTime: Infinity,
  });
}

// ログイン中の利用者。ログインしていなければ data が null
export function useMe() {
  return useQuery({
    queryKey: ME_KEY,
    queryFn: api.getMe,
    staleTime: 5 * 60 * 1000,
  });
}

export function useLogin() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: api.login,
    onSuccess: (res) => {
      // 別の利用者でログインし直したときに、前のお気に入りを出さない
      clearUserData(client);
      client.setQueryData(ME_KEY, res.user);
    },
  });
}

// ログアウトと退会は、Google の自動ログインも止める（次に開いたときに勝手にログインしないように）
export function useLogout() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      await forgetThisDevice(true);
      await api.logout();
    },
    onSuccess: () => {
      window.google?.accounts.id.disableAutoSelect();
      client.setQueryData(ME_KEY, null);
      clearUserData(client);
    },
  });
}

export function useDeleteAccount() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: async () => {
      await api.deleteAccount();
      await forgetThisDevice(false);
    },
    onSuccess: () => {
      window.google?.accounts.id.disableAutoSelect();
      client.setQueryData(ME_KEY, null);
      clearUserData(client);
    },
  });
}

// Google Identity Services（https://developers.google.com/identity/gsi/web）のうち、使うものだけの型
export interface GoogleCredentialResponse {
  credential: string;
}

interface GoogleAccountsId {
  initialize(options: {
    client_id: string;
    callback: (res: GoogleCredentialResponse) => void;
    ux_mode?: "popup" | "redirect";
  }): void;
  renderButton(
    parent: HTMLElement,
    options: {
      theme?: "outline" | "filled_blue" | "filled_black";
      size?: "large" | "medium" | "small";
      text?: "signin_with" | "signup_with" | "continue_with" | "signin";
      shape?: "rectangular" | "pill";
      locale?: string;
    },
  ): void;
  disableAutoSelect(): void;
}

declare global {
  interface Window {
    google?: { accounts: { id: GoogleAccountsId } };
  }
}

const GSI_SRC = "https://accounts.google.com/gsi/client";

let gsiPromise: Promise<GoogleAccountsId> | null = null;

// Google Identity Services のスクリプトを1回だけ読み込む。ログインの画面を開いたときだけ読む
export function loadGoogleIdentity(): Promise<GoogleAccountsId> {
  if (window.google?.accounts.id) {
    return Promise.resolve(window.google.accounts.id);
  }

  gsiPromise ??= new Promise((resolve, reject) => {
    const script = document.createElement("script");
    script.src = GSI_SRC;
    script.async = true;
    script.onload = () => {
      if (window.google?.accounts.id) {
        resolve(window.google.accounts.id);
      } else {
        reject(new Error("Google Identity Services is not available"));
      }
    };
    script.onerror = () => {
      // 次に開いたときに読み直せるようにする
      gsiPromise = null;
      script.remove();
      reject(new Error("failed to load Google Identity Services"));
    };
    document.head.appendChild(script);
  });

  return gsiPromise;
}
