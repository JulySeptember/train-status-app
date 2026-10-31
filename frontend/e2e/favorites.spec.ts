import { type Favorites } from "../src/types.ts";

import { GOTANDA, gotanda } from "./data.ts";
import { expect, test } from "./mock.ts";

const stationPage = `/stations/${encodeURIComponent(GOTANDA)}`;

const NO_FAVORITES: Favorites = { stations: [], railways: [], journeys: [] };

// 応答を返すまで待たせる。resolve を呼ぶと応答を返す
function deferred() {
  let resolve = () => {};
  const promise = new Promise<void>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

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
  const remove = page.getByRole("button", {
    name: "五反田駅をお気に入りから外す",
  });

  // 星はお気に入りを読み込むまで押せない。読み込みが済んでから、再取得の応答を差し替える
  await expect(add).toBeEnabled();
  await expect(add).toHaveAttribute("aria-pressed", "false");

  // 登録したあとの再取得では、登録済みを返す
  api.set("/me/favorites", { body: { ...NO_FAVORITES, stations: [GOTANDA] } });
  await add.click();

  // 押せるようになるのは、登録と再取得が済んでから
  await expect(remove).toBeEnabled();
  await expect(remove).toHaveAttribute("aria-pressed", "true");
  expect(api.sent).toEqual([`PUT /me/favorites/stations/${GOTANDA}`]);

  api.set("/me/favorites", { body: NO_FAVORITES });
  await remove.click();

  await expect(add).toBeEnabled();
  await expect(add).toHaveAttribute("aria-pressed", "false");
  expect(api.sent).toEqual([
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
  const star = page.getByRole("button", { name: /^五反田駅をお気に入り/ });
  await expect(star).toBeEnabled();

  // 失敗したあとの再取得を止めておき、再取得ではなく失敗の処理で元に戻ることを確かめる
  const refetch = deferred();
  api.set("/me/favorites", { body: NO_FAVORITES, wait: refetch.promise });
  await star.click();

  // 押した直後は登録済みの表示にし、失敗したら（再取得を待たずに）元に戻す
  await expect.poll(() => api.sent).toHaveLength(1);
  await expect(star).toHaveAccessibleName("五反田駅をお気に入りに登録");
  await expect(star).toHaveAttribute("aria-pressed", "false");

  // 失敗の理由は、再取得が済んでから出る（mutation は onSettled の再取得を待つ）
  refetch.resolve();
  await expect(page.getByRole("alert")).toHaveText(
    "お気に入りは50件まで登録できます。",
  );
  await expect(star).toHaveAttribute("aria-pressed", "false");
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
