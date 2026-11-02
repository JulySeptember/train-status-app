import { ASAKUSA, GOTANDA, asakusaStations, gotanda } from "./data.ts";
import { expect, test } from "./mock.ts";

test.beforeEach(({ api }) => {
  api.set(`/routes/${ASAKUSA}/stations`, { body: asakusaStations });
  api.set(`/stations/${GOTANDA}`, { body: gotanda });
});

test("路線: 駅を路線の順に並べ、選んだ方向で駅の時刻表を開く", async ({
  page,
}) => {
  await page.goto(`/routes/${encodeURIComponent(ASAKUSA)}`);

  await expect(
    page.getByRole("heading", { level: 1, name: "浅草線" }),
  ).toBeVisible();
  await expect(page.getByRole("main").getByRole("listitem")).toHaveText(
    asakusaStations.map((s) => s.name),
  );

  // 方向を選ぶと URL に残る
  const southbound = page.getByRole("radio", { name: "西馬込方面" });
  await expect(page.getByRole("radio", { name: "押上方面" })).toBeChecked();
  // 選んだ方向は URL を変えてから反映されるので、check() ではなく押してから選ばれるのを待つ
  await southbound.click();
  await expect(southbound).toBeChecked();
  await expect(page).toHaveURL(/direction=odpt\.RailDirection%3ASouthbound/);

  await page.getByRole("link", { name: "五反田" }).click();

  await expect(page).toHaveURL(
    `/stations/${encodeURIComponent(GOTANDA)}?direction=${encodeURIComponent("odpt.RailDirection:Southbound")}`,
  );
  await expect(
    page.getByRole("heading", { level: 1, name: "五反田" }),
  ).toBeVisible();
  await expect(page.getByRole("tab", { name: "西馬込方面" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
});
