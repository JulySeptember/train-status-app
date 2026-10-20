import { useState } from "react";
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Info, LocateFixed } from "lucide-react";

import { api } from "@/api";

import Loading from "@/components/Loading";
import Error from "@/components/Error";
import RailwayBadge from "@/components/RailwayBadge";

import { Button } from "@/components/ui/button";

import {
  LOW_ACCURACY_M,
  nearbyStations,
  walkMinutes,
  type Position,
} from "@/lib/nearby";
import { useRailways } from "@/lib/railways";
import { stationPath } from "@/lib/station";

// 位置情報を取れなかったときの案内（GeolocationPositionError の code ごと）
const LOCATE_ERRORS: Record<number, string> = {
  1: "位置情報の利用が許可されていません。ブラウザの設定で許可してください。",
  2: "現在地を取得できませんでした。",
  3: "現在地の取得に時間がかかっています。もう一度お試しください。",
};

type Locate =
  | { state: "idle" }
  | { state: "locating" }
  | { state: "located"; here: Position; accuracy: number }
  | { state: "error"; message: string };

// 現在地に近い駅。位置情報はボタンを押したときにだけ取り、サーバーには送らない（全駅の座標から画面で探す）
export default function NearbyStations() {
  const [locate, setLocate] = useState<Locate>({ state: "idle" });
  const railways = useRailways();
  const stations = useQuery({
    queryKey: ["stations"],
    queryFn: api.getAllStations,
  });

  const start = () => {
    if (!("geolocation" in navigator)) {
      setLocate({
        state: "error",
        message: "このブラウザでは位置情報を使えません。",
      });
      return;
    }
    setLocate({ state: "locating" });
    navigator.geolocation.getCurrentPosition(
      (pos) =>
        setLocate({
          state: "located",
          here: { lat: pos.coords.latitude, lon: pos.coords.longitude },
          accuracy: pos.coords.accuracy,
        }),
      (err) =>
        setLocate({
          state: "error",
          message: LOCATE_ERRORS[err.code] ?? LOCATE_ERRORS[2],
        }),
      { timeout: 10_000, maximumAge: 60_000 },
    );
  };

  let body;
  if (locate.state === "located") {
    if (stations.isPending) {
      body = <Loading />;
    } else if (stations.error) {
      body = <Error />;
    } else {
      const nearby = nearbyStations(stations.data, locate.here);
      body =
        nearby.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            近く（3km 以内）に駅がありません。
          </p>
        ) : (
          <ul className="grid gap-3 md:grid-cols-2">
            {nearby.map((n) => (
              <li
                key={n.stations[0].id}
                className="space-y-2 rounded-xl border bg-card px-4 py-3"
              >
                <p className="flex flex-wrap items-baseline gap-x-3">
                  <span className="font-semibold">{n.name}</span>
                  <span className="text-sm text-muted-foreground">
                    約{Math.round(n.distance / 10) * 10}m・徒歩
                    {walkMinutes(n.distance)}分
                  </span>
                </p>

                <ul className="flex flex-wrap gap-2">
                  {n.stations.map((st) => {
                    const railway = railways.data?.find(
                      (r) => r.id === st.railwayId,
                    );
                    return (
                      <li key={st.id}>
                        <Link
                          to={stationPath(st.id)}
                          className="inline-flex items-center gap-1.5 rounded-full border px-2 py-1 text-sm transition hover:bg-muted"
                        >
                          <RailwayBadge
                            railway={railway}
                            className="size-5 text-[10px]"
                          />
                          {railway?.name ?? "時刻表"}
                        </Link>
                      </li>
                    );
                  })}
                </ul>
              </li>
            ))}
          </ul>
        );
      // PC のブラウザなどでは、Wi-Fi や IP からの推定で数百m〜数kmずれることがある
      if (locate.accuracy > LOW_ACCURACY_M) {
        body = (
          <>
            <p className="flex items-start gap-1.5 text-sm text-muted-foreground">
              <Info size={14} className="mt-0.5 shrink-0" />
              現在地の精度が低いため（誤差 約
              {Math.round(locate.accuracy / 100) / 10}km）、距離は目安です。
            </p>
            {body}
          </>
        );
      }
    }
  } else if (locate.state === "error") {
    body = (
      <p className="text-sm text-destructive-foreground">{locate.message}</p>
    );
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <h2 className="text-2xl font-bold">近くの駅</h2>

        <Button
          variant="outline"
          size="sm"
          disabled={locate.state === "locating"}
          onClick={start}
        >
          <LocateFixed />
          {locate.state === "locating"
            ? "現在地を取得しています"
            : locate.state === "located"
              ? "現在地を取り直す"
              : "現在地から探す"}
        </Button>
      </div>

      {body}

      {locate.state === "idle" && (
        <p className="text-xs text-muted-foreground">
          現在地は駅を探すためだけに使い、サーバーには送りません。
        </p>
      )}
    </section>
  );
}
