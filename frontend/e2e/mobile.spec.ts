import { GOTANDA, gotanda } from "./data.ts";
import { expect, test } from "./mock.ts";

// スマホの幅（playwright.config.ts の mobile project だけで動かす）。
// PC とは描き方が違うところ（画面下のタブバー、時刻表をタブで1つずつ出す）を確かめる

test.beforeEach(({ api }) => {
  api.set(`/stations/${GOTANDA}`, { body: gotanda });
});

test("スマホ: 画面下のタブバーで画面を移る", async ({ page }) => {
  await page.goto("/");

  // ヘッダーのナビゲーションは隠し、画面下のタブバーだけを出す
  const nav = page.getByRole("navigation", { name: "メイン" });
  await expect(nav).toHaveCount(1);
  await expect(nav.getByRole("link")).toHaveCount(4);

  await nav.getByRole("link", { name: "駅・時刻表" }).click();
  await expect(page).toHaveURL("/stations");
  await expect(nav.getByRole("link", { name: "駅・時刻表" })).toHaveAttribute(
    "aria-current",
    "page",
  );
});

test("スマホ: 時刻表はダイヤをタブで1つずつ出し、本日のダイヤを最初に開く", async ({
  page,
}) => {
  await page.goto(`/stations/${encodeURIComponent(GOTANDA)}`);

  const weekday = page.getByRole("tab", { name: "平日" });
  const holiday = page.getByRole("tab", { name: "土休日" });
  await expect(weekday).toHaveAttribute("aria-selected", "true");

  // 現在は 08:00。押上方面の次発は 08:03
  await expect(page.getByRole("heading", { level: 3 })).toHaveText(["平日"]);
  await expect(
    page.getByRole("link").filter({ hasText: "次発" }),
  ).toContainText("08:03");

  await holiday.click();
  await expect(holiday).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("heading", { level: 3 })).toHaveText(["土休日"]);
  await expect(page.getByRole("main")).toContainText("08:06");
  await expect(page.getByText("次発")).toHaveCount(0);
});
