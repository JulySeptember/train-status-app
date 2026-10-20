import { ASAKUSA_DAIMON, OEDO_DAIMON, allStations } from "./data.ts";
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
  // 位置が正確なら、目安だとは出さない
  await expect(section).not.toContainText("目安");

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

test("近くの駅: 同じ名前でも600mより離れた駅は、別の駅として出す", async ({
  page,
  context,
  api,
}) => {
  // 大門駅（浅草線）と、約1km 離れた同じ名前の架空の駅
  const far = "odpt.Station:Toei.Oedo.FarDaimon";
  api.set("/stations", {
    body: [
      allStations.find((s) => s.id === ASAKUSA_DAIMON),
      {
        ...allStations.find((s) => s.id === OEDO_DAIMON),
        id: far,
        lat: 35.6656,
      },
    ],
  });
  await context.grantPermissions(["geolocation"]);
  await context.setGeolocation({ latitude: 35.6566, longitude: 139.7548 });

  await page.goto("/");
  await page.getByRole("button", { name: "現在地から探す" }).click();

  const items = page.getByRole("listitem").filter({ hasText: /徒歩/ });
  await expect(items).toHaveCount(2);
  await expect(items.first().getByRole("link")).toHaveAttribute(
    "href",
    `/stations/${encodeURIComponent(ASAKUSA_DAIMON)}`,
  );
  await expect(items.nth(1).getByRole("link")).toHaveAttribute(
    "href",
    `/stations/${encodeURIComponent(far)}`,
  );
});

test("近くの駅: 現在地の精度が低ければ、距離は目安だと知らせる", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["geolocation"]);
  await context.setGeolocation({
    latitude: 35.6555,
    longitude: 139.7565,
    accuracy: 2000,
  });

  await page.goto("/");
  await page.getByRole("button", { name: "現在地から探す" }).click();

  await expect(
    page.getByText("現在地の精度が低いため（誤差 約2km）、距離は目安です。"),
  ).toBeVisible();
  await expect(
    page.getByRole("listitem").filter({ hasText: /徒歩/ }).first(),
  ).toContainText("大門");
});

test("近くの駅: 駅・時刻表の画面でも出し、駅名で絞り込んでいるあいだは隠す", async ({
  page,
  context,
}) => {
  await context.grantPermissions(["geolocation"]);
  await context.setGeolocation({ latitude: 35.6555, longitude: 139.7565 });

  await page.goto("/stations");
  await page.getByRole("button", { name: "現在地から探す" }).click();

  const section = page.locator("section", {
    has: page.getByRole("heading", { name: "近くの駅" }),
  });
  await expect(
    section.getByRole("listitem").filter({ hasText: /徒歩/ }).first(),
  ).toContainText("大門");

  const search = page.getByRole("searchbox", { name: "駅名" });
  await search.fill("戸越");
  await expect(
    page.getByRole("heading", { name: "近くの駅" }),
  ).not.toBeVisible();

  // 検索欄を空に戻すと、取り直さずに前の結果を出す
  await search.fill("");
  await expect(
    section.getByRole("listitem").filter({ hasText: /徒歩/ }).first(),
  ).toContainText("大門");
  await expect(
    section.getByRole("button", { name: "現在地を取り直す" }),
  ).toBeVisible();
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
