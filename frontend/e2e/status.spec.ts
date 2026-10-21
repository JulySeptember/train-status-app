import { ASAKUSA, OEDO, asakusaStations, railways, statuses } from "./data.ts";
import { expect, test } from "./mock.ts";

test("運行情報: 平常の路線と遅れのある路線を見分けて出す", async ({
  page,
  api,
}) => {
  api.set(`/routes/${ASAKUSA}/stations`, { body: asakusaStations });

  await page.goto("/");

  await expect(
    page.getByText("三田線で遅れが出ています。", { exact: true }),
  ).toBeVisible();

  const asakusa = page.getByRole("link", { name: /^浅草線/ });
  await expect(asakusa).toContainText("平常運転");

  // 「１５分以上の遅延はありません」も平常として扱う
  await expect(page.getByRole("link", { name: /^大江戸線/ })).toContainText(
    "平常運転",
  );

  const mita = page.getByRole("link", { name: /^三田線/ });
  await expect(mita).toContainText("人身事故の影響で");
  await expect(mita).not.toContainText("平常運転");

  // 路線を選ぶと駅の一覧を開く
  await asakusa.click();
  await expect(page).toHaveURL(`/routes/${encodeURIComponent(ASAKUSA)}`);
});

test("運行情報: すべて平常なら、その旨を出す", async ({ page, api }) => {
  api.set("/status", {
    body: statuses.map((s) => ({
      ...s,
      status: "現在、平常どおり運転しています。",
    })),
  });

  await page.goto("/");

  await expect(
    page.getByText("すべての路線で、15分以上の遅れはありません。"),
  ).toBeVisible();
});

test("運行情報: 事業者が複数なら、遅れのある事業者だけを開く", async ({
  page,
  api,
}) => {
  const ginza = "odpt.Railway:TokyoMetro.Ginza";
  api.set("/routes", {
    body: [
      ...railways,
      {
        id: ginza,
        name: "銀座線",
        lineCode: "G",
        color: "#F39700",
        operator: "odpt.Operator:TokyoMetro",
        operatorName: "東京メトロ",
      },
    ],
  });
  api.set("/status", {
    body: [
      ...statuses.map((s) => ({
        ...s,
        status: "現在、平常どおり運転しています。",
      })),
      {
        railwayId: ginza,
        railway: "銀座線",
        status: "車両点検の影響で、遅れが出ています。",
      },
    ],
  });

  await page.goto("/");

  const metro = page.getByRole("button", { name: /^東京メトロ/ });
  const toei = page.getByRole("button", { name: /^都営交通/ });

  await expect(metro).toContainText("1路線で遅れ");
  await expect(metro).toHaveAttribute("aria-expanded", "true");
  await expect(toei).toContainText("平常運転");
  await expect(toei).toHaveAttribute("aria-expanded", "false");

  // 閉じている事業者を開くと、路線が出る
  await expect(page.getByRole("link", { name: /^浅草線/ })).toBeHidden();
  await toei.click();
  await expect(page.getByRole("link", { name: /^浅草線/ })).toBeVisible();
});

test("運行情報: 終電のあとの路線は、遅れとも平常運転とも出さない", async ({
  page,
  api,
}) => {
  const ginza = "odpt.Railway:TokyoMetro.Ginza";
  const tx = "odpt.Railway:MIR.TsukubaExpress";
  const ended = "ただいまの時間は運行を終了しております。";
  api.set("/routes", {
    body: [
      ...railways,
      {
        id: ginza,
        name: "銀座線",
        lineCode: "G",
        color: "#F39700",
        operator: "odpt.Operator:TokyoMetro",
        operatorName: "東京メトロ",
      },
      {
        id: tx,
        name: "つくばエクスプレス線",
        lineCode: "TX",
        color: "#000084",
        operator: "odpt.Operator:MIR",
        operatorName: "つくばエクスプレス",
      },
    ],
  });
  api.set("/status", {
    body: [
      // 同じ事業者の中で時間外と平常が混ざるときは、見出しを平常運転にする
      ...statuses.map((s) => ({
        ...s,
        status:
          s.railwayId === OEDO ? ended : "現在、平常どおり運転しています。",
      })),
      { railwayId: ginza, railway: "銀座線", status: ended },
      {
        railwayId: tx,
        railway: "つくばエクスプレス線",
        status: "情報提供時間は4:00～翌2:00となっています。",
      },
    ],
  });

  await page.goto("/");

  await expect(
    page.getByText("すべての路線で、15分以上の遅れはありません。"),
  ).toBeVisible();

  const metro = page.getByRole("button", { name: /^東京メトロ/ });
  await expect(metro).toContainText("運行時間外");
  await expect(metro).toHaveAttribute("aria-expanded", "false");

  await metro.click();
  const line = page.getByRole("link", { name: /^銀座線/ });
  await expect(line).toContainText(ended);
  await expect(line).not.toContainText("平常運転");

  await expect(
    page.getByRole("button", { name: /^つくばエクスプレス/ }),
  ).toContainText("運行時間外");

  const toei = page.getByRole("button", { name: /^都営交通/ });
  await expect(toei).toContainText("平常運転");
  await toei.click();
  await expect(page.getByRole("link", { name: /^大江戸線/ })).toContainText(
    ended,
  );
});

test("運行情報: 事業者の運行情報を取れなかった路線は、遅れとも平常とも出さない", async ({
  page,
  api,
}) => {
  const ginza = "odpt.Railway:TokyoMetro.Ginza";
  const unavailable = {
    status: "運行情報を取得できませんでした",
    unavailable: true,
  };
  api.set("/routes", {
    body: [
      ...railways,
      {
        id: ginza,
        name: "銀座線",
        lineCode: "G",
        color: "#F39700",
        operator: "odpt.Operator:TokyoMetro",
        operatorName: "東京メトロ",
      },
    ],
  });
  api.set("/status", {
    body: [
      ...statuses.map((s) =>
        s.railwayId === OEDO ? { ...s, ...unavailable } : s,
      ),
      { railwayId: ginza, railway: "銀座線", ...unavailable },
    ],
  });

  await page.goto("/");

  // 遅れの案内には、取れなかった路線を含めない
  await expect(
    page.getByText("三田線で遅れが出ています。", { exact: true }),
  ).toBeVisible();

  const metro = page.getByRole("button", { name: /^東京メトロ/ });
  await expect(metro).toContainText("取得できませんでした");
  await expect(metro).toHaveAttribute("aria-expanded", "false");

  const toei = page.getByRole("button", { name: /^都営交通/ });
  await expect(toei).toContainText("1路線で遅れ");
  await expect(toei).toHaveAttribute("aria-expanded", "true");

  const oedo = page.getByRole("link", { name: /^大江戸線/ });
  await expect(oedo).toContainText("運行情報を取得できませんでした");
  await expect(oedo).not.toContainText("平常運転");
});

test("運行情報: 取得に失敗したらエラーを出す", async ({ page, api }) => {
  api.set("/status", { status: 500, body: { error: "internal" } });

  await page.goto("/");

  await expect(page.getByText("データの取得に失敗しました。")).toBeVisible();
  await expect(page.getByRole("button", { name: "再読み込み" })).toBeVisible();
});
