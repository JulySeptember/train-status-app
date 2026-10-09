import type {
  ChatErrorBody,
  ChatMessage,
  ChatResponse,
  Fare,
  JourneyQuery,
  JourneySearch,
  Railway,
  Station,
  StationDetail,
  StationSummary,
  TrainLocation,
  TrainStatus,
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

  getFare(from: string, to: string) {
    return request<Fare>(
      `/fares?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
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
};
