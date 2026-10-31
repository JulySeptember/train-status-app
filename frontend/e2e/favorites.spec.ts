import { GOTANDA, gotanda } from "./data.ts";
import { expect, test } from "./mock.ts";

const stationPage = `/stations/${encodeURIComponent(GOTANDA)}`;

test.beforeEach(({ api }) => {
  api.set(`/stations/${GOTANDA}`, { body: gotanda });
});

test("お気に入り: ログインしていなければ、星を押すとアカウントの画面へ案内する", async ({
  page,
  api,
}) => {
  // ログインは使えるが、ログインしていない
  api.set("/auth/config", { body: { googleClientId: "e2e" } });

  // アカウントの画面が読み込む Google のログインのスクリプトは取りに行かない
  await page.route("https://accounts.google.com/**", (route) => route.abort());
  await page.goto(stationPage);

  await page
    .getByRole("button", {
      name: "五反田駅をお気に入りに登録（ログインが必要です）",
    })
    .click();
  await expect(page).toHaveURL("/account");
});

test("お気に入り: 駅を登録・解除すると PUT・DELETE を送り、星の表示が変わる", async ({
  page,
  api,
}) => {
  api.login();
  api.set(`/me/favorites/stations/${GOTANDA}`, { status: 204 }, "PUT");
  api.set(`/me/favorites/stations/${GOTANDA}`, { status: 204 }, "DELETE");

  await page.goto(stationPage);

  const add = page.getByRole("button", { name: "五反田駅をお気に入りに登録" });
  await expect(add).toHaveAttribute("aria-pressed", "false");

  // 登録したあとの再取得では、登録済みを返す
  api.login({ stations: [GOTANDA], railways: [], journeys: [] });
  await add.click();

  const remove = page.getByRole("button", {
    name: "五反田駅をお気に入りから外す",
  });
  await expect(remove).toHaveAttribute("aria-pressed", "true");
  await expect
    .poll(() => api.sent)
    .toEqual([`PUT /me/favorites/stations/${GOTANDA}`]);

  api.login();
  await remove.click();

  await expect(add).toHaveAttribute("aria-pressed", "false");
  await expect
    .poll(() => api.sent)
    .toEqual([
      `PUT /me/favorites/stations/${GOTANDA}`,
      `DELETE /me/favorites/stations/${GOTANDA}`,
    ]);
});

test("お気に入り: 登録に失敗したら、星を元に戻して理由を出す", async ({
  page,
  api,
}) => {
  api.login();
  api.set(
    `/me/favorites/stations/${GOTANDA}`,
    { status: 409, body: { error: "お気に入りは50件まで登録できます。" } },
    "PUT",
  );

  await page.goto(stationPage);
  await page
    .getByRole("button", { name: "五反田駅をお気に入りに登録" })
    .click();

  await expect(page.getByRole("alert")).toHaveText(
    "お気に入りは50件まで登録できます。",
  );
  await expect(
    page.getByRole("button", { name: "五反田駅をお気に入りに登録" }),
  ).toHaveAttribute("aria-pressed", "false");
});

test("お気に入り: ホームに登録した駅を出す", async ({ page, api }) => {
  api.login({ stations: [GOTANDA], railways: [], journeys: [] });

  await page.goto("/");

  const section = page.locator("section", {
    has: page.getByRole("heading", { name: "お気に入り" }),
  });
  await expect(section.getByRole("link", { name: /五反田/ })).toHaveAttribute(
    "href",
    stationPage,
  );
});
