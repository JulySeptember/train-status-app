import type {
  Alert,
  AuthConfig,
  ChatErrorBody,
  ChatMessage,
  ChatResponse,
  FavoriteKind,
  Favorites,
  JourneyQuery,
  JourneySearch,
  LoginResponse,
  PushConfig,
  Railway,
  Station,
  StationDetail,
  StationSummary,
  TrainLocation,
  TrainStatus,
  User,
} from "./types";

const API = "/api";

// HttpError は GET の API が成功以外のステータスを返したときのエラー。
// 指定の誤り（4xx）とサーバー側の失敗（5xx）で、画面の文言を分けるのに使う
export class HttpError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

// 4xx（指定の誤り）か。通信の失敗（fetch の TypeError）や 5xx は false
export function isClientError(error: unknown) {
  return (
    error instanceof HttpError && error.status >= 400 && error.status < 500
  );
}

// Query は TanStack Query の queryFn に渡される値のうち、使うもの。
// signal を渡すと、条件が変わったときや画面を離れたときに前のリクエストを取り消せる
type Query = { signal?: AbortSignal };

async function request<T>(url: string, { signal }: Query = {}): Promise<T> {
  const res = await fetch(`${API}${url}`, { signal });

  if (!res.ok) {
    throw new HttpError(res.status, await res.text());
  }

  return await res.json();
}

// status のときは null を返す GET（ログインしていない 401、設定が無い 503 など）
async function requestOrNull<T>(
  url: string,
  status: number,
  { signal }: Query = {},
): Promise<T | null> {
  const res = await fetch(`${API}${url}`, { signal });

  if (res.status === status) {
    return null;
  }
  if (!res.ok) {
    throw new HttpError(res.status, await res.text());
  }

  return await res.json();
}

// ChatError は /api/chat のエラー。message はユーザーに見せる文言
export class ChatError extends Error {
  code: string;

  constructor(body: ChatErrorBody) {
    super(body.error);
    this.code = body.code;
  }
}

async function postChat(messages: ChatMessage[]): Promise<ChatResponse> {
  const res = await fetch(`${API}/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ messages }),
  });

  if (!res.ok) {
    // CloudFront や API Gateway のエラーは JSON でないことがある
    const body = await res.json().catch(() => null);
    throw new ChatError(
      body?.error && body?.code
        ? body
        : { error: "エラーが発生しました。", code: "internal" },
    );
  }

  return await res.json();
}

// send は状態を変えるリクエスト（ログイン・ログアウト・退会・お気に入り）。本文の無いレスポンス（204）は undefined を返す。
// 失敗したときの message は、ユーザーに見せられる文言（バックエンドの error）にする
async function send<T>(
  method: string,
  url: string,
  body?: unknown,
): Promise<T> {
  const res = await fetch(`${API}${url}`, {
    method,
    headers:
      body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });

  if (!res.ok) {
    // CloudFront や API Gateway のエラーは JSON でないことがある
    const err = await res.json().catch(() => null);
    throw new Error(
      typeof err?.error === "string" ? err.error : "エラーが発生しました。",
    );
  }

  return res.status === 204 ? (undefined as T) : await res.json();
}

// ログインしていなければ null を返す
function getMe(query?: Query): Promise<User | null> {
  return requestOrNull<User>("/me", 401, query);
}

// セッションが切れている（401）ときは null を返す
function getFavorites(query?: Query): Promise<Favorites | null> {
  return requestOrNull<Favorites>("/me/favorites", 401, query);
}

// ログインを使えない（バックエンドに設定が無い）ときは null を返す
function getAuthConfig(query?: Query): Promise<AuthConfig | null> {
  return requestOrNull<AuthConfig>("/auth/config", 503, query);
}

// 経路の id は「出発駅ID~到着駅ID」。ID は : を含むので、それぞれ encodeURIComponent する
function favoritePath(kind: FavoriteKind, id: string) {
  return `/me/favorites/${kind}/${id.split("~").map(encodeURIComponent).join("~")}`;
}

export const api = {
  getStatus(query?: Query) {
    return request<TrainStatus[]>("/status", query);
  },

  getRoutes(query?: Query) {
    return request<Railway[]>("/routes", query);
  },

  getStations(routeId: string, query?: Query) {
    return request<Station[]>(
      `/routes/${encodeURIComponent(routeId)}/stations`,
      query,
    );
  },

  getAllStations(query?: Query) {
    return request<StationSummary[]>("/stations", query);
  },

  getStation(stationId: string, query?: Query) {
    return request<StationDetail>(
      `/stations/${encodeURIComponent(stationId)}`,
      query,
    );
  },

  getTrain(trainId: string, query?: Query) {
    return request<TrainLocation>(
      `/trains/${encodeURIComponent(trainId)}/location`,
      query,
    );
  },

  searchJourneys(query: JourneyQuery, options?: Query) {
    const params = new URLSearchParams({ from: query.from, to: query.to });

    if (query.departAt) params.set("departAt", query.departAt);
    if (query.arriveBy) params.set("arriveBy", query.arriveBy);
    if (query.maxTransfers !== undefined) {
      params.set("maxTransfers", String(query.maxTransfers));
    }
    if (query.avoid?.length) params.set("avoid", query.avoid.join(","));
    if (query.realtime === false) params.set("realtime", "false");

    return request<JourneySearch>(`/journeys?${params}`, options);
  },

  chat(messages: ChatMessage[]) {
    return postChat(messages);
  },

  getAuthConfig,

  getMe,

  login(credential: string) {
    return send<LoginResponse>("POST", "/auth/google", { credential });
  },

  logout() {
    return send<void>("POST", "/auth/logout");
  },

  deleteAccount() {
    return send<void>("DELETE", "/me");
  },

  getFavorites,

  addFavorite(kind: FavoriteKind, id: string) {
    return send<void>("PUT", favoritePath(kind, id));
  },

  removeFavorite(kind: FavoriteKind, id: string) {
    return send<void>("DELETE", favoritePath(kind, id));
  },

  // 通知を使えない（VAPID の鍵が無い）ときは null を返す
  getPushConfig(query?: Query): Promise<PushConfig | null> {
    return requestOrNull<PushConfig>("/push/config", 503, query);
  },

  getAlerts(query?: Query) {
    return request<Alert[]>("/me/alerts", query);
  },

  setAlert({ railwayId, ...body }: Alert) {
    return send<void>(
      "PUT",
      `/me/alerts/${encodeURIComponent(railwayId)}`,
      body,
    );
  },

  removeAlert(railwayId: string) {
    return send<void>("DELETE", `/me/alerts/${encodeURIComponent(railwayId)}`);
  },

  // subscription は PushSubscription.toJSON() の形
  subscribePush(subscription: PushSubscriptionJSON) {
    return send<void>("POST", "/me/push-subscriptions", subscription);
  },

  // この端末がログイン中の利用者の購読に登録されているか（endpoint は URL に入れない）
  subscriptionStatus(endpoint: string) {
    return send<{ registered: boolean }>(
      "POST",
      "/me/push-subscriptions/status",
      { endpoint },
    );
  },

  unsubscribePush(endpoint: string) {
    return send<void>("DELETE", "/me/push-subscriptions", { endpoint });
  },

  sendTestNotification() {
    return send<{ sent: number }>("POST", "/me/push-subscriptions/test");
  },
};
