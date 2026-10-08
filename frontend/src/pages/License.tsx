import { type ReactNode } from "react";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-3">
      <h2 className="text-xl font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function ExternalLink({
  href,
  children,
}: {
  href: string;
  children: ReactNode;
}) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="text-brand underline-offset-4 hover:underline"
    >
      {children}
    </a>
  );
}

export default function License() {
  return (
    <div className="max-w-3xl space-y-8 leading-relaxed">
      <div className="space-y-3">
        <h1 className="text-3xl font-bold">データ提供・ライセンス</h1>

        <p className="text-foreground/80">
          このページは東京都交通局が公開するオープンデータを加工して利用しています。
        </p>
      </div>

      <Section title="データ提供者">
        <p className="text-foreground/80">
          東京都交通局・公共交通オープンデータ協議会
        </p>
      </Section>

      <Section title="利用データ">
        <ul className="list-disc space-y-1 pl-6 text-foreground/80">
          <li>列車位置情報</li>
          <li>列車運行情報</li>
          <li>路線情報</li>
          <li>駅情報</li>
          <li>駅時刻表</li>
          <li>列車時刻表</li>
          <li>運賃情報</li>
          <li>乗降者数情報</li>
        </ul>
      </Section>

      <Section title="AI の利用について">
        <p className="text-foreground/80">
          「AI に聞く」では、Google の Gemini API を使っています。入力内容は AI
          の提供元（Google）に送信され、サービス改善に利用されることがあります。個人情報は入力しないでください。
        </p>

        <p>
          <ExternalLink href="https://ai.google.dev/gemini-api/terms">
            Gemini API 利用規約
          </ExternalLink>
        </p>

        <p className="text-foreground/80">
          AI
          の回答は誤ることがあります。駅・時刻・経路・運行状況は、アプリが持つオープンデータから取得しています。
        </p>
      </Section>

      <Section title="ライセンス">
        <p className="text-foreground/80">
          このアプリは東京都交通局・公共交通オープンデータ協議会が提供するオープンデータを改変して利用しています。
        </p>

        <p className="text-foreground/80">
          ライセンス：
          <ExternalLink href="https://creativecommons.org/licenses/by/4.0/deed.ja">
            Creative Commons Attribution 4.0 International (CC BY 4.0)
          </ExternalLink>
        </p>
      </Section>
    </div>
  );
}
