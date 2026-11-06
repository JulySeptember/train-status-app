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

// 駅のボタン（role="combobox"）。名前は「出発駅を選択」か、選んだあとは「出発駅: 五反田」
function stationButton(page: Page, label: "出発駅" | "到着駅") {
  return page.getByRole("combobox", { name: new RegExp(`^${label}`) });
}

// 駅を選ぶ（ボタンを押し、駅名で絞り込んで選ぶ）
async function selectStation(
  page: Page,
  label: "出発駅" | "到着駅",
  name: string,
) {
  await stationButton(page, label).click();
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
  await page.getByRole("radio", { name: "出発" }).check();
  await page.getByLabel("出発時刻").fill("09:00");
  await page.getByRole("radio", { name: "今すぐ" }).check();
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

test("経路検索: 結果を乗り換えの少ない順・所要時間の短い順に並べ替え、並び順を URL に残す", async ({
  page,
  api,
}) => {
  const base = journeySearch.journeys[0];
  // API の並び（早く着く順）・乗り換えの少ない順・所要時間の短い順が、すべて違う3本
  api.set("/journeys", {
    body: {
      ...journeySearch,
      journeys: [
        {
          ...base,
          departureTime: "08:05",
          arrivalTime: "08:30",
          transfers: 2,
        },
        {
          ...base,
          departureTime: "08:12",
          arrivalTime: "08:34",
          transfers: 1,
        },
        {
          ...base,
          departureTime: "08:00",
          arrivalTime: "08:38",
          transfers: 0,
        },
      ],
    },
  });

  const badges = page.getByText(/^乗り換え(なし|\d回)$/);
  const order = ["乗り換え2回", "乗り換え1回", "乗り換えなし"];

  await page.goto(resultUrl);
  await expect(badges).toHaveText(order);
  await expect(page.getByRole("radio", { name: "早く着く順" })).toBeChecked();

  let searches = 0;
  page.on("request", (r) => {
    if (new URL(r.url()).pathname === "/api/journeys") {
      searches++;
    }
  });

  // 並び順は URL に持たせていて、押したあと少し遅れて選ばれるので、check() ではなく click() で押す
  await page.getByRole("radio", { name: "乗り換えが少ない順" }).click();
  await expect(badges).toHaveText([
    "乗り換えなし",
    "乗り換え1回",
    "乗り換え2回",
  ]);
  await expect(page).toHaveURL(`${resultUrl}&sort=transfers`);

  await page.getByRole("radio", { name: "所要時間が短い順" }).click();
  await expect(badges).toHaveText([
    "乗り換え1回",
    "乗り換え2回",
    "乗り換えなし",
  ]);
  await expect(page).toHaveURL(`${resultUrl}&sort=duration`);
  // 並べ替えだけでは検索し直さない
  expect(searches).toBe(0);

  // 再読み込みしても並び順を保つ
  await page.reload();
  await expect(badges).toHaveText([
    "乗り換え1回",
    "乗り換え2回",
    "乗り換えなし",
  ]);
  await expect(
    page.getByRole("radio", { name: "所要時間が短い順" }),
  ).toBeChecked();

  // 検索し直しても並び順を保つ
  await page.getByRole("radio", { name: "出発" }).check();
  await page.getByLabel("出発時刻").fill("09:00");
  await page.getByRole("button", { name: "検索" }).click();
  await expect(page).toHaveURL(/departAt=09%3A00/);
  await expect(page).toHaveURL(/sort=duration/);
  await expect(badges).toHaveText([
    "乗り換え1回",
    "乗り換え2回",
    "乗り換えなし",
  ]);

  // 既定の並びに戻すと URL から消える
  await page.getByRole("radio", { name: "早く着く順" }).click();
  await expect(badges).toHaveText(order);
  await expect(page).not.toHaveURL(/sort=/);
});

test("経路検索: 到着時刻の指定では、既定の並び順を遅く出る順と出す", async ({
  page,
  api,
}) => {
  const base = journeySearch.journeys[0];
  api.set("/journeys", {
    body: {
      ...journeySearch,
      journeys: [base, { ...base, departureTime: "08:00", transfers: 0 }],
    },
  });

  await page.goto(`${resultUrl}&arriveBy=09%3A00`);
  await expect(page.getByRole("radio", { name: "遅く出る順" })).toBeChecked();
  await expect(page.getByRole("radio", { name: "早く着く順" })).toHaveCount(0);
});

test("経路検索: 経路が1本なら並び順を出さない", async ({ page, api }) => {
  api.set("/journeys", { body: journeySearch });

  await page.goto(resultUrl);
  await expect(page.getByText("乗り換え1回")).toBeVisible();
  await expect(page.getByRole("radiogroup", { name: "並び順" })).toHaveCount(0);
});

test("経路検索: 遅延を反映しない・到着時刻を指定した条件で検索する", async ({
  page,
  api,
}) => {
  api.set("/journeys", { body: journeySearch });

  await page.goto("/journeys");

  await selectStation(page, "出発駅", "五反田");
  await selectStation(page, "到着駅", "六本木");
  await page.getByRole("radio", { name: "到着" }).check();
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
  await expect(page.getByRole("radio", { name: "到着" })).toBeChecked();
  await expect(page.getByLabel("到着時刻")).toHaveValue("09:00");
  await expect(page.getByLabel("遅延・運転見合わせを反映")).not.toBeChecked();
});

test("経路検索: ひらがな・カタカナ・ローマ字でも駅を探せる", async ({
  page,
}) => {
  await page.goto("/journeys");
  await stationButton(page, "出発駅").click();
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
  // ボタンの名前は、選んだ駅だけでなく出発駅か到着駅かも含む
  await expect(stationButton(page, "出発駅")).toHaveAccessibleName(
    "出発駅: 五反田",
  );
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
  await stationButton(page, "出発駅").click();
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
  await expect(stationButton(page, "出発駅")).toContainText("五反田");
  await expect(stationButton(page, "到着駅")).toContainText("大門");
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

  await expect(stationButton(page, "出発駅")).toContainText("大門");
  await expect(stationButton(page, "到着駅")).toContainText("大門");
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

test("経路検索: 降りる駅の中の案内（何両目に乗るか・どこへ歩くか）を乗り換えに出す", async ({
  page,
  api,
}) => {
  const [first, second] = journeySearch.journeys[0].legs;
  api.set("/journeys", {
    body: {
      ...journeySearch,
      journeys: [
        {
          ...journeySearch.journeys[0],
          legs: [
            {
              ...first,
              alightGuide: {
                carFromFront: 3,
                cars: 8,
                toward: "front",
                towardMeters: 30,
                turn: "right",
                via: "escalator",
                target: "大江戸線 1番線",
                walkMeters: 150,
                walkMinutes: 2,
              },
            },
            second,
          ],
        },
      ],
    },
  });

  await page.goto(resultUrl);

  await expect(
    page.getByText(
      "前から3両目付近（8両編成） → 降りて右（進行方向）へ約30m → エスカレーター → 大江戸線 1番線（約150m・徒歩2分）",
    ),
  ).toBeVisible();
});

test("経路検索: 何両目か分からない駅では、ホームを出る場所と歩く先を出す", async ({
  page,
  api,
}) => {
  const [first, second] = journeySearch.journeys[0].legs;
  api.set("/journeys", {
    body: {
      ...journeySearch,
      journeys: [
        {
          ...journeySearch.journeys[0],
          legs: [
            {
              ...first,
              alightGuide: {
                exitAt: "back",
                via: "stairs",
                target: "南改札",
                walkMeters: 60,
                walkMinutes: 1,
              },
            },
            second,
          ],
        },
      ],
    },
  });

  await page.goto(resultUrl);

  await expect(
    page.getByText("ホームの後ろ寄りの階段 → 南改札（約60m・徒歩1分）"),
  ).toBeVisible();
});

test("経路検索: 同じ名前の離れた駅で乗り降りするなら、路線名で見分けて徒歩の区間を出す", async ({
  page,
  api,
}) => {
  const [leg] = journeySearch.journeys[0].legs;
  api.set("/journeys", {
    body: {
      ...journeySearch,
      journeys: [
        {
          departureTime: "10:00",
          arrivalTime: "10:20",
          transfers: 0,
          walkBeforeMinutes: 0,
          walkAfterMinutes: 11,
          legs: [
            {
              ...leg,
              departureTime: "10:00",
              arrivalTime: "10:09",
              delayMinutes: 0,
            },
          ],
        },
      ],
    },
  });

  // 到着駅に、降りる駅（浅草線の大門）と同じ名前の大江戸線の大門を指定する
  await page.goto(
    `/journeys?from=${encodeURIComponent(GOTANDA)}&to=${encodeURIComponent(OEDO_DAIMON)}`,
  );

  await expect(
    page.getByText("浅草線の大門からほかの路線の大門まで徒歩11分"),
  ).toBeVisible();
});

test("経路検索: 同じ名前の離れた駅から乗るなら、路線名で見分けて徒歩の区間を出す", async ({
  page,
  api,
}) => {
  const [leg] = journeySearch.journeys[0].legs;
  api.set("/journeys", {
    body: {
      ...journeySearch,
      journeys: [
        {
          departureTime: "09:49",
          arrivalTime: "10:09",
          transfers: 0,
          walkBeforeMinutes: 11,
          walkAfterMinutes: 0,
          legs: [
            {
              ...leg,
              from: ASAKUSA_DAIMON,
              fromName: "大門",
              to: GOTANDA,
              toName: "五反田",
              departureTime: "10:00",
              arrivalTime: "10:09",
              delayMinutes: 0,
            },
          ],
        },
      ],
    },
  });

  // 出発駅に、乗る駅（浅草線の大門）と同じ名前の大江戸線の大門を指定する
  await page.goto(
    `/journeys?from=${encodeURIComponent(OEDO_DAIMON)}&to=${encodeURIComponent(GOTANDA)}`,
  );

  await expect(
    page.getByText("ほかの路線の大門から浅草線の大門まで徒歩11分"),
  ).toBeVisible();
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

test("経路検索: 3時前に朝の時刻で探したら、その朝からの時刻表どおりの結果だと知らせる", async ({
  page,
  api,
}) => {
  api.set("/journeys", {
    body: { ...journeySearch, delayApplied: false, nextServiceDay: true },
  });

  await page.goto(resultUrl);

  await expect(
    page.getByText(
      "今日の朝からの時刻表どおりの結果です（遅れや運転見合わせは反映していません）。",
    ),
  ).toBeVisible();
  await expect(page.getByText("運行状況を取得できなかったため")).toHaveCount(0);
});

test("経路検索: 指定の誤り（4xx）なら、指定を確かめるよう案内する", async ({
  page,
  api,
}) => {
  api.set("/journeys", { status: 400, body: { error: "bad request" } });

  await page.goto(resultUrl);

  await expect(page.getByText("経路を検索できませんでした。")).toBeVisible();
  await expect(page.getByText("駅や時刻の指定を確かめて")).toBeVisible();
});

test("経路検索: サーバーのエラー（5xx）なら、指定ではなく時間をおくよう案内する", async ({
  page,
  api,
}) => {
  api.set("/journeys", { status: 500, body: { error: "internal" } });

  await page.goto(resultUrl);

  // 1回失敗したあと、再試行（retry: 1）を待ってから失敗になる
  await expect(page.getByText("経路を検索できませんでした。")).toBeVisible({
    timeout: 10_000,
  });
  await expect(
    page.getByText("時間をおいて、もう一度検索してください。"),
  ).toBeVisible();
  await expect(page.getByText("駅や時刻の指定を確かめて")).toHaveCount(0);
});

test("経路検索: 流量の制限（429）でも、指定ではなく時間をおくよう案内する", async ({
  page,
  api,
}) => {
  api.set("/journeys", { status: 429, body: { message: "Too Many Requests" } });

  await page.goto(resultUrl);

  await expect(page.getByText("経路を検索できませんでした。")).toBeVisible({
    timeout: 10_000,
  });
  await expect(
    page.getByText("時間をおいて、もう一度検索してください。"),
  ).toBeVisible();
});

test("経路検索: 時刻の指定は矢印キーで選べる", async ({ page }) => {
  await page.goto("/journeys");

  const now = page.getByRole("radio", { name: "今すぐ" });
  await now.focus();
  await page.keyboard.press("ArrowRight");

  await expect(page.getByRole("radio", { name: "出発" })).toBeChecked();
  await expect(page.getByLabel("出発時刻")).toBeVisible();
});

test("経路検索: 結果を待つ間に画面を離れたら、検索のリクエストを取り消す", async ({
  page,
  api,
}) => {
  // 応答を返さないまま待たせる
  api.set("/journeys", { body: journeySearch, wait: new Promise(() => {}) });

  const requests: string[] = [];
  const failed: string[] = [];
  const isJourneys = (url: string) => new URL(url).pathname === "/api/journeys";
  page.on("request", (r) => isJourneys(r.url()) && requests.push(r.url()));
  page.on("requestfailed", (r) => isJourneys(r.url()) && failed.push(r.url()));

  await page.goto(resultUrl);
  await expect(page.getByText("読み込み中...")).toBeVisible();
  // 開発時の StrictMode は一度アンマウントして最初のリクエストを取り消すので、待っているリクエストが残るのを待つ
  await expect.poll(() => requests.length - failed.length).toBe(1);

  await page
    .getByRole("navigation", { name: "メイン" })
    .getByRole("link", { name: "運行情報" })
    .click();

  await expect.poll(() => requests.length - failed.length).toBe(0);
});
