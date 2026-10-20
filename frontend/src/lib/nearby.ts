import { type StationSummary } from "@/types";

// 近くの駅として出す最大の距離と駅の数
const MAX_DISTANCE_M = 3000;
const MAX_STATIONS = 5;

// 徒歩の速さ（不動産の表示と同じ分速80m）
const WALK_M_PER_MIN = 80;

const EARTH_RADIUS_M = 6_371_000;

export type Position = { lat: number; lon: number };

// 近くの駅。同じ名前の駅（路線ごとに別の駅）は1つにまとめる
export type NearbyStation = {
  name: string;
  // いちばん近い駅までの距離（m）
  distance: number;
  // 近い順の、その名前の駅
  stations: StationSummary[];
};

// 2点間の距離（m）。近い駅を比べるだけなので、平面に近似する（バックエンドの service/transfer.go と同じ）
export function distance(a: Position, b: Position) {
  const lat1 = (a.lat * Math.PI) / 180;
  const lat2 = (b.lat * Math.PI) / 180;
  const dLat = lat2 - lat1;
  const dLon =
    (((b.lon - a.lon) * Math.PI) / 180) * Math.cos((lat1 + lat2) / 2);
  return EARTH_RADIUS_M * Math.hypot(dLat, dLon);
}

export function walkMinutes(meters: number) {
  return Math.max(1, Math.ceil(meters / WALK_M_PER_MIN));
}

// 現在地から近い順に駅を返す。座標の無い駅と、遠すぎる駅は除く
export function nearbyStations(
  stations: StationSummary[],
  here: Position,
): NearbyStation[] {
  const sorted = stations
    .filter((s) => s.lat !== 0 || s.lon !== 0)
    .map((s) => ({ station: s, distance: distance(here, s) }))
    .filter((s) => s.distance <= MAX_DISTANCE_M)
    .sort((a, b) => a.distance - b.distance);

  const byName = new Map<string, NearbyStation>();
  for (const { station, distance } of sorted) {
    const found = byName.get(station.name);
    if (found) {
      found.stations.push(station);
    } else {
      byName.set(station.name, {
        name: station.name,
        distance,
        stations: [station],
      });
    }
  }

  return [...byName.values()].slice(0, MAX_STATIONS);
}
