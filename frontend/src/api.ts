import type {
  AuthConfig,
  ChatErrorBody,
  ChatMessage,
  ChatResponse,
  JourneyQuery,
  JourneySearch,
  LoginResponse,
  Railway,
  Station,
  StationDetail,
  StationSummary,
  TrainLocation,
  TrainStatus,
  User,
} from "./types";

const API = "/api";

async function request<T>(url: string): Promise<T> {
  const res = await fetch(`${API}${url}`);

  if (!res.ok) {
    throw new Error(await res.text());
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

// send は状態を変えるリクエスト（ログイン・ログアウト・退会）。本文の無いレスポンス（204）は undefined を返す
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
    throw new Error(await res.text());
  }

  return res.status === 204 ? (undefined as T) : await res.json();
}

// ログインしていなければ null を返す
async function getMe(): Promise<User | null> {
  const res = await fetch(`${API}/me`);

  if (res.status === 401) {
    return null;
  }
  if (!res.ok) {
    throw new Error(await res.text());
  }

  return await res.json();
}

// ログインを使えない（バックエンドに設定が無い）ときは null を返す
async function getAuthConfig(): Promise<AuthConfig | null> {
  const res = await fetch(`${API}/auth/config`);

  if (res.status === 503) {
    return null;
  }
  if (!res.ok) {
    throw new Error(await res.text());
  }

  return await res.json();
}

export const api = {
  getStatus() {
    return request<TrainStatus[]>("/status");
  },

  getRoutes() {
    return request<Railway[]>("/routes");
  },

  getStations(routeId: string) {
    return request<Station[]>(
      `/routes/${encodeURIComponent(routeId)}/stations`,
    );
  },

  getAllStations() {
    return request<StationSummary[]>("/stations");
  },

  getStation(stationId: string) {
    return request<StationDetail>(`/stations/${encodeURIComponent(stationId)}`);
  },

  getTrain(trainId: string) {
    return request<TrainLocation>(
      `/trains/${encodeURIComponent(trainId)}/location`,
    );
  },

  searchJourneys(query: JourneyQuery) {
    const params = new URLSearchParams({ from: query.from, to: query.to });

    if (query.departAt) params.set("departAt", query.departAt);
    if (query.arriveBy) params.set("arriveBy", query.arriveBy);
    if (query.maxTransfers !== undefined) {
      params.set("maxTransfers", String(query.maxTransfers));
    }
    if (query.avoid?.length) params.set("avoid", query.avoid.join(","));
    if (query.realtime === false) params.set("realtime", "false");

    return request<JourneySearch>(`/journeys?${params}`);
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
};
