import { ASAKUSA_DAIMON, OEDO_DAIMON } from "./data.ts";
import { expect, test } from "./mock.ts";

test("近くの駅: 現在地から近い順に、同じ名前の駅をまとめて出す", async ({
  page,
  context,
}) => {
  // 大門駅の近く（浜松町）
  await context.grantPermissions(["geolocation"]);
  await context.setGeolocation({ latitude: 35.6555, longitude: 139.7565 });

  await page.goto("/");

  const section = page.locator("section", {
    has: page.getByRole("heading", { name: "近くの駅" }),
  });
  await section.getByRole("button", { name: "現在地から探す" }).click();

  const items = section.getByRole("listitem").filter({ hasText: /徒歩/ });
  await expect(items.first()).toContainText("大門");
  await expect(items.nth(1)).toContainText("三田");
  // 3km より遠い西馬込は出さない
  await expect(section).not.toContainText("西馬込");

  // 大門は浅草線・大江戸線の両方の時刻表を開ける
  const daimon = items.first();
  await expect(daimon.getByRole("link")).toHaveCount(2);
  await expect(daimon.getByRole("link", { name: /大江戸線/ })).toHaveAttribute(
    "href",
    `/stations/${encodeURIComponent(OEDO_DAIMON)}`,
  );
  await expect(daimon.getByRole("link", { name: /浅草線/ })).toHaveAttribute(
    "href",
    `/stations/${encodeURIComponent(ASAKUSA_DAIMON)}`,
  );
});

test("近くの駅: 位置情報が許可されていなければ、その旨を出す", async ({
  page,
}) => {
  await page.goto("/");

  await page.getByRole("button", { name: "現在地から探す" }).click();

  await expect(
    page.getByText("位置情報の利用が許可されていません。", { exact: false }),
  ).toBeVisible();
});

test("近くの駅: 近くに駅が無ければ、その旨を出す", async ({
  page,
  context,
}) => {
  // 大阪駅
  await context.grantPermissions(["geolocation"]);
  await context.setGeolocation({ latitude: 34.7025, longitude: 135.4959 });

  await page.goto("/");
  await page.getByRole("button", { name: "現在地から探す" }).click();

  await expect(
    page.getByText("近く（3km 以内）に駅がありません。"),
  ).toBeVisible();
});
