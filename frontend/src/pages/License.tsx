import { type ReactNode } from "react";
import PageTitle from "@/components/PageTitle";
import { groupByOperator, useRailways } from "@/lib/railways";

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

const CC_BY = {
  name: "クリエイティブ・コモンズ 表示 4.0 国際（CC BY 4.0）",
  url: "https://creativecommons.org/licenses/by/4.0/deed.ja",
};

const BASIC = {
  name: "公共交通オープンデータ基本ライセンス",
  url: "https://developer.odpt.org/terms/data_basic_license.html",
};

const CHALLENGE = {
  name: "公共交通オープンデータチャレンジ限定ライセンス",
  url: "https://developer.odpt.org/challenge_license",
};

// 事業者ごとのデータのライセンス（docs/design/multi-operator.md 10章）
const LICENSES: Record<string, typeof CC_BY> = {
  "odpt.Operator:Toei": CC_BY,
  "odpt.Operator:TokyoMetro": BASIC,
  "odpt.Operator:TWR": BASIC,
  "odpt.Operator:MIR": BASIC,
  "odpt.Operator:TamaMonorail": BASIC,
  "odpt.Operator:Yurikamome": BASIC,
  "odpt.Operator:JR-East": CHALLENGE,
  "odpt.Operator:Keio": CHALLENGE,
  "odpt.Operator:Tobu": CHALLENGE,
  "odpt.Operator:Keikyu": CHALLENGE,
  "odpt.Operator:Tokyu": CHALLENGE,
  "odpt.Operator:Seibu": CHALLENGE,
  "odpt.Operator:Odakyu": CHALLENGE,
};

const CONTACT_URL = "https://github.com/JulySeptember/train-status-app/issues";

// このアプリが使っている事業者と、そのデータのライセンス
function Providers() {
  const railways = useRailways();

  // 路線一覧を取得できないときも、都営の表示は出す
  const operators = railways.data
    ? groupByOperator(railways.data, (r) => r)
    : [{ operator: "odpt.Operator:Toei", name: "都営交通", items: [] }];

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-left text-sm">
        <thead className="text-muted-foreground">
          <tr className="border-b">
            <th className="py-2 pr-4 font-medium">事業者</th>
            <th className="py-2 font-medium">ライセンス</th>
          </tr>
        </thead>

        <tbody className="text-foreground/80">
          {operators.map((op) => {
            const license = LICENSES[op.operator];

            return (
              <tr key={op.operator} className="border-b last:border-0">
                <td className="py-2 pr-4">{op.name}</td>
                <td className="py-2">
                  {license ? (
                    <ExternalLink href={license.url}>
                      {license.name}
                    </ExternalLink>
                  ) : (
                    "—"
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

export default function License() {
  return (
    <div className="max-w-3xl space-y-8 leading-relaxed">
      <PageTitle title="データ提供・ライセンス" />
      <div className="space-y-3">
        <h1 className="text-3xl font-bold">データ提供・ライセンス</h1>

        <p className="text-foreground/80">
          このアプリは、公共交通オープンデータセンターを通じて各鉄道事業者が公開しているオープンデータを加工して利用しています。
        </p>
      </div>

      <Section title="データ提供者">
        <p className="text-foreground/80">
          公共交通オープンデータ協議会（公共交通オープンデータセンター）と、次の事業者のデータを使っています。
        </p>

        <Providers />
      </Section>

      <Section title="利用データ">
        <ul className="list-disc space-y-1 pl-6 text-foreground/80">
          <li>列車位置情報</li>
          <li>列車運行情報</li>
          <li>路線情報</li>
          <li>駅情報</li>
          <li>駅時刻表</li>
          <li>列車時刻表</li>
          <li>運賃情報（都営のみ）</li>
          <li>乗降者数情報（都営のみ）</li>
        </ul>
      </Section>

      <Section title="免責事項">
        <p className="text-foreground/80">
          このアプリは個人が開発した非公式のアプリで、公共交通オープンデータセンター・各事業者が提供・保証するものではありません。表示する運行情報・時刻表・経路などの正確性・完全性・即時性は保証しません。実際の運行は各事業者の案内をご確認ください。
        </p>

        <p className="text-foreground/80">
          東急電鉄・西武鉄道・小田急電鉄・京急電鉄・ゆりかもめは、列車ごとの時刻表が公開されていないため、駅の時刻表の発車をつないで列車の運行を推定し、経路検索に使っています。発車時刻は駅の時刻表のとおりですが、どの発車が同じ列車か（乗り換えずに行けるか）と到着時刻は推定で、実際と異なることがあります。
        </p>

        <p className="text-foreground/80">
          このアプリについて、公共交通オープンデータセンター・各事業者へのお問い合わせはご遠慮ください。
        </p>
      </Section>

      <Section title="お問い合わせ">
        <p className="text-foreground/80">
          このアプリへのお問い合わせ・不具合の報告は、
          <ExternalLink href={CONTACT_URL}>GitHub の Issues</ExternalLink>
          で受け付けています。
        </p>
      </Section>

      <Section title="AI の利用について">
        <p className="text-foreground/80">
          「AI 乗換相談」では、Google の Gemini API を使っています。入力内容は
          AI
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
    </div>
  );
}
