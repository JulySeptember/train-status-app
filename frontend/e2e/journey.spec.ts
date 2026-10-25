import { type Page } from "@playwright/test";

import { type StationSummary } from "../src/types.ts";

import {
  ASAKUSA_DAIMON,
  GOTANDA,
  MITA,
  OEDO_DAIMON,
  ROPPONGI,
  allStations,
  journeySearch,
} from "./data.ts";
import { expect, test } from "./mock.ts";

// 駅を選ぶ（ボタンを押し、駅名で絞り込んで選ぶ）。
// 駅のボタン（role="combobox"）には名前が付いていないので、並び順（出発駅・到着駅）で引く
async function selectStation(
  page: Page,
  label: "出発駅" | "到着駅",
  name: string,
) {
  await page
    .getByRole("combobox")
    .nth(label === "出発駅" ? 0 : 1)
    .click();
  await page.getByPlaceholder(`${label}を検索...`).fill(name);
  await page.getByRole("option", { name, exact: true }).click();
  // 閉じるアニメーションのあいだは選択肢が残るので、次の駅を選ぶ前に閉じるのを待つ
  await expect(page.getByRole("option")).toHaveCount(0);
}

// 経路検索の API へのリクエストを待つ（検索の条件を確かめる）
async function waitForJourneys(page: Page) {
  const request = await page.waitForRequest(
    (r) => new URL(r.url()).pathname === "/api/journeys",
  );
  return new URL(request.url());
}

const resultUrl = `/journeys?from=${encodeURIComponent(GOTANDA)}&to=${encodeURIComponent(ROPPONGI)}`;

test("経路検索: ホームから駅を選んで検索し、乗り換えと遅れを含む経路を出す", async ({
  page,
  api,
}) => {
  api.set("/journeys", { body: journeySearch });

  await page.goto("/");
  await page.getByRole("tab", { name: "経路検索" }).click();

  const search = page.getByRole("button", { name: "検索" });
  await expect(search).toBeDisabled();

  await selectStation(page, "出発駅", "五反田");
  await selectStation(page, "到着駅", "六本木");
  // 時刻を入れてから「今すぐ」に戻したときも、時刻は送らない
  await page.getByRole("tab", { name: "出発" }).click();
  await page.getByLabel("出発時刻").fill("09:00");
  await page.getByRole("tab", { name: "今すぐ" }).click();
  const request = waitForJourneys(page);
  await search.click();

  await expect(page).toHaveURL(resultUrl);

  const params = (await request).searchParams;
  expect(params.get("from")).toBe(GOTANDA);
  expect(params.get("to")).toBe(ROPPONGI);
  expect(params.has("realtime")).toBe(false);
  // 「今すぐ」は時刻を送らない（バックエンドが現在時刻で探す）
  expect(params.has("departAt")).toBe(false);
  expect(params.has("arriveBy")).toBe(false);

  await expect(page.getByText("08:0508:26")).toBeVisible();
  await expect(page.getByText("21分", { exact: true })).toBeVisible();
  await expect(page.getByText("乗り換え1回")).toBeVisible();
  await expect(page.getByText("約2分遅れ")).toBeVisible();
  await expect(
    page.getByText("大門（大江戸線）へ乗り換え・待ち 5分"),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "列車番号 803T" }),
  ).toHaveAttribute(
    "href",
    `/trains/${encodeURIComponent("odpt.Train:Toei.Asakusa.803T")}`,
  );
});

test("経路検索: 遅延を反映しない・到着時刻を指定した条件で検索する", async ({
  page,
  api,
}) => {
  api.set("/journeys", { body: journeySearch });

  await page.goto("/journeys");

  await selectStation(page, "出発駅", "五反田");
  await selectStation(page, "到着駅", "六本木");
  await page.getByRole("tab", { name: "到着" }).click();
  await page.getByLabel("到着時刻").fill("09:00");
  await page.getByLabel("遅延・運転見合わせを反映").uncheck();
  const request = waitForJourneys(page);
  await page.getByRole("button", { name: "検索" }).click();

  await expect(page.getByText("乗り換え1回")).toBeVisible();

  const params = (await request).searchParams;
  expect(params.get("arriveBy")).toBe("09:00");
  expect(params.get("realtime")).toBe("false");

  // 条件は URL に残り、再読み込みしても入力欄に戻る
  await page.reload();
  await expect(page.getByRole("tab", { name: "到着" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.getByLabel("到着時刻")).toHaveValue("09:00");
  await expect(page.getByLabel("遅延・運転見合わせを反映")).not.toBeChecked();
});

test("経路検索: ひらがな・カタカナ・ローマ字でも駅を探せる", async ({
  page,
}) => {
  await page.goto("/journeys");
  await page.getByRole("combobox").first().click();
  const input = page.getByPlaceholder("出発駅を検索...");
  const option = (name: string) =>
    page.getByRole("option", { name, exact: true });

  // 促音（ろっぽんぎ の っ）・カタカナ・大文字・英語名の前方の一致
  for (const keyword of ["ろっぽんぎ", "ロッポンギ", "Roppongi", "ropp"]) {
    await input.fill(keyword);
    await expect(page.getByRole("option")).toHaveCount(1);
    await expect(option("六本木")).toBeVisible();
  }

  // 「まごめ」は馬込（Magome）・西馬込（Nishi-magome）に当たり、名前が検索語で始まる馬込を先に出す
  await input.fill("まごめ");
  await expect(page.getByRole("option")).toHaveCount(2);
  await expect(page.getByRole("option").nth(0)).toHaveAccessibleName("馬込");
  await expect(page.getByRole("option").nth(1)).toHaveAccessibleName("西馬込");

  // 漢字を含む検索語は駅名だけと比べる
  await input.fill("五反だ");
  await expect(page.getByText("駅が見つかりません")).toBeVisible();

  await input.fill("ごたんだ");
  await option("五反田").click();
  await expect(page.getByRole("combobox").first()).toContainText("五反田");
});

test("経路検索: 拗音・長音・b の前の m・英単語の駅名も、かなとローマ字で探せる", async ({
  page,
  api,
}) => {
  // 英語名に長音の記号（他社の駅データの書き方）や mb を含む駅、英語名が英単語の駅を足す
  const extra = (id: string, name: string, nameEn: string): StationSummary => ({
    id: `odpt.Station:Toei.Mita.${id}`,
    name,
    railwayId: MITA,
    nameEn,
    journeyStation: `odpt.Station:Toei.Mita.${id}`,
    lat: 0,
    lon: 0,
  });
  api.set("/stations", {
    body: [
      ...allStations,
      extra("Jimbocho", "神保町", "Jimbocho"),
      extra("Shimbashi", "新橋", "Shimbashi"),
      extra("ShakujiiKoen", "石神井公園", "Shakujii-kōen"),
      extra("Hatchobori", "八丁堀", "Hatchobori"),
      extra("TelecomCenter", "テレコムセンター", "Telecom Center"),
      extra("ShimuraSakaue", "志村坂上", "Shimura-sakaue"),
    ],
  });

  await page.goto("/journeys");
  await page.getByRole("combobox").first().click();
  const input = page.getByPlaceholder("出発駅を検索...");

  const cases: [string, string][] = [
    // ちょう（拗音と ou の長音）と、Jimbocho の m
    ["じんぼうちょう", "神保町"],
    // 半角カナ・全角英字
    ["ｼﾞﾝﾎﾞｳﾁｮｳ", "神保町"],
    ["Ｊｉｍｂｏ", "神保町"],
    // しゃ と、英語名の長音の記号（ō）
    ["しゃくじいこうえん", "石神井公園"],
    ["shakujiikoen", "石神井公園"],
    // っち → tch
    ["はっちょうぼり", "八丁堀"],
    // カタカナの駅名は、ひらがなでも駅名と比べる（英語名とは合わない）
    ["てれこむせんたー", "テレコムセンター"],
  ];
  for (const [keyword, name] of cases) {
    await input.fill(keyword);
    await expect(page.getByRole("option")).toHaveCount(1);
    await expect(page.getByRole("option", { name, exact: true })).toBeVisible();
  }

  // 打っている途中の「shim」は、m のままの志村坂上（Shimura）と、b の前で n になる新橋（Shimbashi）の両方を出す
  for (const keyword of ["shim", "しm"]) {
    await input.fill(keyword);
    await expect(
      page.getByRole("option", { name: "新橋", exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("option", { name: "志村坂上", exact: true }),
    ).toBeVisible();
  }
  await input.fill("shimb");
  await expect(page.getByRole("option")).toHaveCount(1);
  await expect(
    page.getByRole("option", { name: "新橋", exact: true }),
  ).toBeVisible();

  // 「m」だけでも、n を含まない三田（Mita）を出す
  await input.fill("m");
  await expect(
    page.getByRole("option", { name: "三田", exact: true }),
  ).toBeVisible();
});

test("経路検索: 出発駅と到着駅が同じなら検索できない", async ({ page }) => {
  await page.goto("/journeys");

  await selectStation(page, "出発駅", "大門");
  await selectStation(page, "到着駅", "五反田");
  await expect(page.getByRole("button", { name: "検索" })).toBeEnabled();

  // 入れ替えても検索できる。到着駅を出発駅と同じにすると検索できない
  await page
    .getByRole("button", { name: "出発駅と到着駅を入れ替える" })
    .click();
  await expect(page.getByRole("combobox").nth(0)).toContainText("五反田");
  await expect(page.getByRole("combobox").nth(1)).toContainText("大門");
  await expect(page.getByRole("button", { name: "検索" })).toBeEnabled();
  await selectStation(page, "到着駅", "五反田");
  await expect(page.getByRole("button", { name: "検索" })).toBeDisabled();
});

test("経路検索: 経路検索で1つにまとめた駅どうし（大江戸線と浅草線の大門）は検索できない", async ({
  page,
  api,
}) => {
  // URL で指定されると結果も取りにいくので、空の結果を返す
  api.set("/journeys", {
    body: { journeys: [], delayApplied: true, suspendedRailways: [] },
  });

  await page.goto(
    `/journeys?from=${encodeURIComponent(OEDO_DAIMON)}&to=${encodeURIComponent(ASAKUSA_DAIMON)}`,
  );

  await expect(page.getByRole("combobox").nth(0)).toContainText("大門");
  await expect(page.getByRole("combobox").nth(1)).toContainText("大門");
  await expect(page.getByRole("button", { name: "検索" })).toBeDisabled();
});

test("経路検索: 0時をまたぐ経路の所要時間", async ({ page, api }) => {
  const [leg] = journeySearch.journeys[0].legs;
  api.set("/journeys", {
    body: {
      ...journeySearch,
      journeys: [
        {
          departureTime: "23:50",
          arrivalTime: "00:10",
          transfers: 0,
          walkBeforeMinutes: 0,
          walkAfterMinutes: 0,
          legs: [
            {
              ...leg,
              departureTime: "23:50",
              arrivalTime: "00:10",
              delayMinutes: 0,
            },
          ],
        },
      ],
    },
  });

  await page.goto(resultUrl);

  await expect(page.getByText("乗り換えなし")).toBeVisible();
  await expect(page.getByText("20分", { exact: true })).toBeVisible();
  await expect(page.getByText("20分乗車")).toBeVisible();
});

test("経路検索: 運転見合わせの路線と、経路が無いことを知らせる", async ({
  page,
  api,
}) => {
  api.set("/journeys", {
    body: {
      journeys: [],
      delayApplied: true,
      suspendedRailways: [{ id: MITA, name: "三田線" }],
    },
  });

  await page.goto(resultUrl);

  await expect(
    page.getByText(
      "三田線は運転を見合わせているため、使わない経路を表示しています。",
    ),
  ).toBeVisible();
  await expect(page.getByText("経路が見つかりませんでした。")).toBeVisible();
  await expect(
    page.getByRole("link", { name: "AI 乗換相談" }).last(),
  ).toHaveAttribute("href", "/chat");
});

test("経路検索: 運行状況を取れなかったら、時刻表どおりの結果だと知らせる", async ({
  page,
  api,
}) => {
  api.set("/journeys", { body: { ...journeySearch, delayApplied: false } });

  await page.goto(resultUrl);

  await expect(
    page.getByText("運行状況を取得できなかったため、時刻表どおりの結果です。"),
  ).toBeVisible();
  await expect(page.getByText("乗り換え1回")).toBeVisible();
});

test("経路検索: 検索に失敗したらエラーを出す", async ({ page, api }) => {
  api.set("/journeys", { status: 400, body: { error: "bad request" } });

  await page.goto(resultUrl);

  await expect(page.getByText("経路を検索できませんでした。")).toBeVisible();
});
