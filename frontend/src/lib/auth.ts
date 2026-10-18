import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api } from "@/api";

const ME_KEY = ["me"];

// ログインし直し・ログアウト・退会したら、前の利用者のお気に入り（lib/favorites.ts）を残さない
function clearFavorites(client: ReturnType<typeof useQueryClient>) {
  client.removeQueries({ queryKey: ["favorites"] });
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
      clearFavorites(client);
      client.setQueryData(ME_KEY, res.user);
    },
  });
}

// ログアウトと退会は、Google の自動ログインも止める（次に開いたときに勝手にログインしないように）
export function useLogout() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: api.logout,
    onSuccess: () => {
      window.google?.accounts.id.disableAutoSelect();
      client.setQueryData(ME_KEY, null);
      clearFavorites(client);
    },
  });
}

export function useDeleteAccount() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: api.deleteAccount,
    onSuccess: () => {
      window.google?.accounts.id.disableAutoSelect();
      client.setQueryData(ME_KEY, null);
      clearFavorites(client);
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
