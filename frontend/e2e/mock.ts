import { test as base, expect, type Page } from "@playwright/test";

import { allStations, railways, statuses } from "./data.ts";

// 画面の現在時刻（日本時間の平日の朝）。時刻表の「次発」や「本日」は、この時刻で決まる
export const NOW = new Date("2026-10-19T08:00:00+09:00");

type Reply = {
  status?: number;
  body?: unknown;
  // 応答を返す前に待つ（回答を待つ間の画面を確かめるため）
  wait?: Promise<unknown>;
};

// /api への GET（と post で登録した POST）に固定の応答を返す。登録していないパスへのリクエストは、テストの終わりに失敗にする
// （モックの登録漏れや、画面が想定外の API を呼んだことに気づけるように）
export class MockApi {
  private replies = new Map<string, Reply>();
  private postReplies = new Map<string, Reply>();

  readonly unhandled: string[] = [];

  constructor() {
    // ログインを使わない状態（お気に入りの星を出さない）
    this.set("/auth/config", {
      status: 503,
      body: { error: "unavailable", code: "unavailable" },
    });
    this.set("/me", { status: 401, body: { error: "unauthorized" } });

    this.set("/routes", { body: railways });
    this.set("/stations", { body: allStations });
    this.set("/status", { body: statuses });
  }

  // path は /api を除いたパス（例: /routes/odpt.Railway:Toei.Asakusa/stations）。
  // ID はエンコードしない形で書く
  set(path: string, reply: Reply) {
    this.replies.set(path, reply);
  }

  // POST の応答を登録する
  post(path: string, reply: Reply) {
    this.postReplies.set(path, reply);
  }

  async install(page: Page) {
    await page.route(
      (url) => url.pathname.startsWith("/api/"),
      async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        const path = decodeURIComponent(url.pathname.slice("/api".length));
        const reply =
          request.method() === "GET"
            ? this.replies.get(path)
            : request.method() === "POST"
              ? this.postReplies.get(path)
              : undefined;

        if (!reply) {
          this.unhandled.push(`${request.method()} ${path}`);
          await route.fulfill({ status: 404, json: { error: "not mocked" } });
          return;
        }

        await reply.wait;

        await route.fulfill({
          status: reply.status ?? 200,
          json: reply.body ?? null,
        });
      },
    );
  }
}

export const test = base.extend<{ api: MockApi }>({
  api: [
    async ({ page }, use) => {
      const api = new MockApi();
      await api.install(page);
      // 時計だけを止め、タイマーは普通に進める（React Query の再試行などが動くように）
      await page.clock.setFixedTime(NOW);

      await use(api);

      expect(api.unhandled, "モックしていない API が呼ばれた").toEqual([]);
    },
    { auto: true },
  ],
});

export { expect };
