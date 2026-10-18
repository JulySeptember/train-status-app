import { type ReactNode } from "react";
import { Link } from "react-router-dom";

import PageTitle from "@/components/PageTitle";

const CONTACT_URL = "https://github.com/JulySeptember/train-status-app/issues";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="text-xl font-semibold">{title}</h2>
      <div className="space-y-3 text-foreground/80">{children}</div>
    </section>
  );
}

// ログイン（docs/design/login.md 7章）で集める情報と、その扱い
export default function Privacy() {
  return (
    <div className="max-w-3xl space-y-8 leading-relaxed">
      <PageTitle title="プライバシーポリシー" />
      <div className="space-y-3">
        <h1 className="text-3xl font-bold">プライバシーポリシー</h1>

        <p className="text-foreground/80">
          このアプリ（NORIKAE
          AI、非公式）が、ログインした利用者について保存する情報と、その扱いを説明します。ログインしなくても、運行情報・経路検索・時刻表・AI
          乗換相談はすべて使えます。
        </p>
      </div>

      <Section title="保存する情報">
        <ul className="list-disc space-y-1 pl-6">
          <li>
            Google アカウントの利用者を見分ける番号（Google が発行する ID）
          </li>
          <li>登録した日時</li>
          <li>
            お気に入りに登録した駅・路線・経路（出発駅と到着駅）と、その日時
          </li>
          <li>運行情報の通知の条件（路線・曜日・時間帯）</li>
          <li>
            通知を受け取る端末の情報（ブラウザが発行する通知の宛先と暗号鍵）。5台まで
          </li>
          <li>ログインの状態（セッション）。30 日使わなければ消えます</li>
        </ul>

        <p>
          ログインのときに Google
          から届く情報にはメールアドレス・名前・プロフィール写真も含まれますが、利用者を見分ける番号のほかは使わず、保存もしません。
        </p>
      </Section>

      <Section title="使い道">
        <p>
          ログインした利用者を見分け、お気に入りと運行情報の通知の機能で、登録した内容を利用者ごとに保存し、通知を届けるためだけに使います。広告や分析には使わず、第三者には提供しません。
        </p>
      </Section>

      <Section title="通知の配信">
        <p>
          通知は、お使いのブラウザの提供元の配信サービス（Google・Mozilla・Apple
          など）を通じて届きます。届ける内容（路線名と運行情報）は暗号化して送ります。通知は、アカウントの画面でいつでも止められます。ログアウトすると、その端末には届かなくなります。
        </p>
      </Section>

      <Section title="Cookie">
        <p>
          ログインの状態を保つためだけに Cookie
          を1つ使います。ログアウトすると消えます。
        </p>
      </Section>

      <Section title="保存場所">
        <p>
          Amazon Web Services（東京リージョン）に保存します。ログインには Google
          のサービスを使います。
        </p>
      </Section>

      <Section title="退会と情報の削除">
        <p>
          <Link to="/account" className="text-brand hover:underline">
            アカウント
          </Link>
          の画面から、いつでも退会できます。退会すると、保存した情報はすべて削除します。
        </p>
      </Section>

      <Section title="お問い合わせ">
        <p>
          このポリシーについてのお問い合わせは、
          <a
            href={CONTACT_URL}
            target="_blank"
            rel="noreferrer"
            className="text-brand underline-offset-4 hover:underline"
          >
            GitHub の Issues
          </a>
          で受け付けています。
        </p>
      </Section>

      <p className="text-sm text-muted-foreground">2026年10月18日 制定</p>
    </div>
  );
}
