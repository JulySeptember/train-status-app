import { useState } from "react";
import { Link } from "react-router-dom";

import Error from "@/components/Error";
import GoogleSignInButton from "@/components/GoogleSignInButton";
import Loading from "@/components/Loading";
import NotificationSettings from "@/components/NotificationSettings";
import PageTitle from "@/components/PageTitle";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useAuthConfig,
  useDeleteAccount,
  useLogin,
  useLogout,
  useMe,
} from "@/lib/auth";

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("ja-JP", {
    year: "numeric",
    month: "long",
    day: "numeric",
  });
}

function LoggedOut({ clientId }: { clientId: string }) {
  const login = useLogin();

  return (
    <div className="space-y-4">
      <p className="text-foreground/80">
        Google
        アカウントでログインすると、駅・路線・経路をお気に入りに登録して、ホームからすぐ開けるようになります。登録した路線の運行情報の通知も受け取れます。ログインしなくても、ほかの機能はすべて使えます。
      </p>

      <GoogleSignInButton
        clientId={clientId}
        onCredential={(credential) => login.mutate(credential)}
      />

      {login.isPending && (
        <p className="text-sm text-muted-foreground">ログインしています...</p>
      )}
      {login.isError && (
        <p className="text-sm text-destructive-foreground">
          ログインできませんでした。もう一度お試しください。
        </p>
      )}

      <p className="text-sm text-muted-foreground">
        保存するのは Google
        の利用者を見分ける番号だけで、メールアドレスや名前は保存しません。詳しくは
        <Link to="/privacy" className="text-brand hover:underline">
          プライバシーポリシー
        </Link>
        をご覧ください。
      </p>
    </div>
  );
}

function LoggedIn({ createdAt }: { createdAt: string }) {
  const logout = useLogout();
  const deleteAccount = useDeleteAccount();
  const [confirming, setConfirming] = useState(false);

  return (
    <div className="space-y-6">
      <div className="space-y-1">
        <p className="font-medium">Google アカウントでログインしています</p>
        <p className="text-sm text-muted-foreground">
          {formatDate(createdAt)} に登録
        </p>
      </div>

      <div className="flex flex-wrap gap-2">
        <Button
          variant="outline"
          disabled={logout.isPending}
          onClick={() => logout.mutate()}
        >
          ログアウト
        </Button>

        <Button variant="destructive" onClick={() => setConfirming(true)}>
          退会する
        </Button>
      </div>

      {logout.isError && (
        <p className="text-sm text-destructive-foreground">
          ログアウトできませんでした。もう一度お試しください。
        </p>
      )}

      <NotificationSettings />

      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>退会しますか？</DialogTitle>
            <DialogDescription>
              登録した内容はすべて消え、元に戻せません。
            </DialogDescription>
          </DialogHeader>

          {deleteAccount.isError && (
            <p className="text-sm text-destructive-foreground">
              退会できませんでした。もう一度お試しください。
            </p>
          )}

          <DialogFooter>
            <DialogClose render={<Button variant="outline" />}>
              やめる
            </DialogClose>
            <Button
              variant="destructive"
              disabled={deleteAccount.isPending}
              onClick={() =>
                deleteAccount.mutate(undefined, {
                  onSuccess: () => setConfirming(false),
                })
              }
            >
              退会する
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

export default function Account() {
  const config = useAuthConfig();
  const me = useMe();

  let body;
  if (config.isPending || me.isPending) {
    body = <Loading />;
  } else if (me.isError) {
    body = <Error message="ログインの状態を確かめられませんでした。" />;
  } else if (me.data) {
    body = <LoggedIn createdAt={me.data.createdAt} />;
  } else if (config.data) {
    body = <LoggedOut clientId={config.data.googleClientId} />;
  } else {
    body = (
      <p className="text-foreground/80">
        いまはログインを使えません。時間をおいてお試しください。
      </p>
    );
  }

  return (
    <div className="max-w-3xl space-y-6 leading-relaxed">
      <PageTitle title="アカウント" />
      <h1 className="text-3xl font-bold">アカウント</h1>
      {body}
    </div>
  );
}
