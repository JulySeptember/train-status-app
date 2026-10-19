import { type Page } from "@playwright/test";

import {
  ASAKUSA,
  GOTANDA,
  RUNNING_TRAIN,
  asakusaStations,
  gotanda,
  runningTrain,
} from "./data.ts";
import { expect, test } from "./mock.ts";

test.beforeEach(({ api }) => {
  api.set(`/stations/${GOTANDA}`, { body: gotanda });
});

// 時刻表の1つのダイヤ（平日・土休日）のカード
const card = (page: Page, title: string) =>
  page
    .getByRole("heading", { level: 3, name: title, exact: true })
    .locator("xpath=ancestor::div[contains(@class,'rounded-xl')][1]");

test("駅: 平日と土休日の時刻表を並べ、本日のダイヤに次発の印を付ける", async ({
  page,
}) => {
  await page.goto(`/stations/${encodeURIComponent(GOTANDA)}`);

  await expect(
    page.getByRole("heading", { level: 1, name: "五反田" }),
  ).toBeVisible();
  await expect(page.getByRole("main").getByText("浅草線")).toBeVisible();

  const weekday = card(page, "平日");
  const holiday = card(page, "土休日");

  await expect(weekday).toContainText("本日");
  await expect(holiday).not.toContainText("本日");

  // 現在は 08:00。押上方面の次発は 08:03
  await expect(weekday.getByText("次発")).toHaveCount(1);
  await expect(
    weekday.getByRole("link").filter({ hasText: "次発" }),
  ).toContainText("08:03");
  await expect(weekday).toContainText("快特");
  await expect(weekday).toContainText("成田空港行");

  // 本日でないダイヤは、列車番号が使い回されるので列車位置へのリンクにしない
  await expect(holiday).toContainText("08:06");
  await expect(holiday.getByRole("link")).toHaveCount(0);
  await expect(holiday.getByText("次発")).toHaveCount(0);

  // 方向を切り替えると、その方向の時刻表になる
  await page.getByRole("tab", { name: "西馬込方面" }).click();
  await expect(page).toHaveURL(/direction=odpt\.RailDirection%3ASouthbound/);
  await expect(
    weekday.getByRole("link").filter({ hasText: "次発" }),
  ).toContainText("08:05");
  await expect(holiday).toContainText("08:12");
});

test("駅: 本日のダイヤの列車を選ぶと、列車の位置を開く", async ({
  page,
  api,
}) => {
  api.set(`/trains/${RUNNING_TRAIN}/location`, { body: runningTrain });
  api.set(`/routes/${ASAKUSA}/stations`, { body: asakusaStations });

  await page.goto(`/stations/${encodeURIComponent(GOTANDA)}`);

  await card(page, "平日")
    .getByRole("link")
    .filter({ hasText: "08:03" })
    .click();

  await expect(page).toHaveURL(`/trains/${encodeURIComponent(RUNNING_TRAIN)}`);
  await expect(page.getByRole("heading", { level: 1 })).toContainText("押上行");
});

test("駅: 深夜は0時台の列車を運行日の最後として、次発を決める", async ({
  page,
  api,
}) => {
  // 23:50。運行日は3時で切り替わるので、00:10 の列車は同じ運行日のうちで、次発になる
  await page.clock.setFixedTime(new Date("2026-10-19T23:50:00+09:00"));
  const weekday = gotanda.timetables[0];
  api.set(`/stations/${GOTANDA}`, {
    body: {
      ...gotanda,
      timetables: [
        {
          ...weekday,
          timetables: [
            { ...weekday.timetables[0], time: "05:10" },
            { ...weekday.timetables[1], time: "23:40" },
            { ...weekday.timetables[2], time: "00:10" },
          ],
        },
      ],
    },
  });

  await page.goto(`/stations/${encodeURIComponent(GOTANDA)}`);

  await expect(
    card(page, "平日").getByRole("link").filter({ hasText: "次発" }),
  ).toContainText("00:10");
});

test("駅: 列車位置を配信しない路線は、本日のダイヤでも列車へのリンクにしない", async ({
  page,
  api,
}) => {
  api.set(`/stations/${GOTANDA}`, {
    body: { ...gotanda, trainLocationAvailable: false },
  });

  await page.goto(`/stations/${encodeURIComponent(GOTANDA)}`);

  await expect(
    page.getByText("この路線は列車位置情報が提供されていません。"),
  ).toBeVisible();
  const weekday = card(page, "平日");
  await expect(weekday).toContainText("本日");
  await expect(weekday.getByText("次発")).toHaveCount(1);
  await expect(weekday.getByRole("link")).toHaveCount(0);
});
