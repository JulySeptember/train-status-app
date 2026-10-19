import {
  ASAKUSA,
  NISHI_MAGOME,
  RUNNING_TRAIN,
  WAITING_TRAIN,
  asakusaStations,
  runningTrain,
  waitingTrain,
} from "./data.ts";
import { expect, test } from "./mock.ts";

test.beforeEach(({ api }) => {
  api.set(`/routes/${ASAKUSA}/stations`, { body: asakusaStations });
});

test("列車: 走っている列車の遅れと位置を、路線図に出す", async ({
  page,
  api,
}) => {
  api.set(`/trains/${RUNNING_TRAIN}/location`, { body: runningTrain });

  await page.goto(`/trains/${encodeURIComponent(RUNNING_TRAIN)}`);

  await expect(page.getByRole("heading", { level: 1 })).toHaveText(
    "普通押上行",
  );
  await expect(page.getByText("浅草線・北行・列車番号 803T")).toBeVisible();
  await expect(page.getByText("約2分遅れ")).toBeVisible();
  await expect(page.getByText("戸越 → 五反田 を走行中")).toBeVisible();
  await expect(page.getByText("7:59:45 時点の位置。")).toBeVisible();

  // 路線図は出た駅（戸越）の前後3駅だけを出し、戸越と五反田のあいだに列車を置く
  const diagram = page
    .getByRole("heading", { name: "現在位置" })
    .locator("xpath=..")
    .getByRole("listitem");
  await expect(diagram).toHaveText([
    "西馬込",
    "馬込",
    "中延",
    "戸越",
    "走行中",
    "五反田",
    "高輪台",
    "泉岳寺",
  ]);

  await page.getByRole("button", { name: "全9駅を表示" }).click();
  await expect(diagram).toHaveCount(asakusaStations.length + 1);
});

test("列車: 出発前の列車は、始発駅と発車時刻を出す", async ({ page, api }) => {
  api.set(`/trains/${WAITING_TRAIN}/location`, { body: waitingTrain });

  await page.goto(`/trains/${encodeURIComponent(WAITING_TRAIN)}`);

  await expect(page.getByText("まだ出発していません")).toBeVisible();
  await expect(page.getByRole("link", { name: "西馬込" })).toHaveAttribute(
    "href",
    `/stations/${encodeURIComponent(NISHI_MAGOME)}`,
  );
  await expect(page.getByText("08:15 発")).toBeVisible();
  await expect(
    page.getByText("出発したら、自動で位置を表示します。"),
  ).toBeVisible();
});

test("列車: 見つからない列車はエラーを出す", async ({ page, api }) => {
  api.set(`/trains/${RUNNING_TRAIN}/location`, {
    status: 404,
    body: { error: "train not found" },
  });

  await page.goto(`/trains/${encodeURIComponent(RUNNING_TRAIN)}`);

  await expect(page.getByText("データの取得に失敗しました。")).toBeVisible();
});
