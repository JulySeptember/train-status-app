import type {
  Fare,
  JourneyQuery,
  JourneySearch,
  Railway,
  Station,
  StationDetail,
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

  async getAllStations() {
    const routes = await api.getRoutes();

    const stations = await Promise.all(
      routes.map((route) => api.getStations(route.id)),
    );

    return stations.flat();
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

    return request<JourneySearch>(`/journeys?${params}`);
  },
};
