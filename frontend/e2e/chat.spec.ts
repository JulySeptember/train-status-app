import { type Page } from "@playwright/test";

import { type ChatResponse } from "../src/types.ts";

import { expect, test } from "./mock.ts";

const QUESTION = "三田から大門へ行きたい";

const answer: ChatResponse = {
  reply: "春日から大江戸線で浅草へ行けます。",
  steps: [{ tool: "search_route", label: "経路を探しました" }],
};

// 応答を返すまで待たせる。resolve を呼ぶと応答を返す
function deferred() {
  let resolve = () => {};
  const promise = new Promise<void>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

const nav = (page: Page, name: string) =>
  page.getByRole("navigation", { name: "メイン" }).getByRole("link", { name });

async function ask(page: Page) {
  await page.goto("/chat");
  await page.getByRole("textbox", { name: "質問" }).fill(QUESTION);
  await page.getByRole("button", { name: "送信" }).click();
  await expect(page.getByText("運行状況や時刻表を調べています")).toBeVisible();
}

test("AI 乗換相談: 回答を待つ間に別の画面へ移っても、戻ると回答が入っている", async ({
  page,
  api,
}) => {
  const reply = deferred();
  api.post("/chat", { body: answer, wait: reply.promise });

  await ask(page);

  await nav(page, "駅・時刻表").click();
  await expect(page).toHaveURL("/stations");

  reply.resolve();
  // 回答が届くまで待ってから戻る（画面を離れている間に届いた回答を確かめる）
  await page.waitForResponse("**/api/chat");

  await nav(page, "AI 乗換相談").click();

  await expect(page.locator("p", { hasText: QUESTION })).toHaveCount(1);
  await expect(page.getByText(answer.reply)).toBeVisible();
  await expect(page.getByText("運行状況や時刻表を調べています")).toHaveCount(0);
});

test("AI 乗換相談: 回答を待つ間に戻ると、待っている表示が出て、続けて送れない", async ({
  page,
  api,
}) => {
  const reply = deferred();
  api.post("/chat", { body: answer, wait: reply.promise });

  await ask(page);

  await nav(page, "駅・時刻表").click();
  await nav(page, "AI 乗換相談").click();

  await expect(page.getByText(QUESTION)).toBeVisible();
  await expect(page.getByText("運行状況や時刻表を調べています")).toBeVisible();

  await page.getByRole("textbox", { name: "質問" }).fill("次の質問");
  await expect(page.getByRole("button", { name: "送信" })).toBeDisabled();

  reply.resolve();

  await expect(page.getByText(answer.reply)).toBeVisible();
  await expect(page.getByRole("button", { name: "送信" })).toBeEnabled();
});

test("AI 乗換相談: 画面を離れている間に失敗したら、質問を入力欄に戻す", async ({
  page,
  api,
}) => {
  const reply = deferred();
  api.post("/chat", {
    status: 503,
    body: { error: "ただいま混み合っています。", code: "unavailable" },
    wait: reply.promise,
  });

  await ask(page);

  await nav(page, "駅・時刻表").click();
  await expect(page).toHaveURL("/stations");

  reply.resolve();
  await page.waitForResponse("**/api/chat");

  await nav(page, "AI 乗換相談").click();

  await expect(page.getByRole("textbox", { name: "質問" })).toHaveValue(
    QUESTION,
  );
  // 会話には残さない（入力欄の textarea は数えない）
  await expect(page.locator("p", { hasText: QUESTION })).toHaveCount(0);
});
