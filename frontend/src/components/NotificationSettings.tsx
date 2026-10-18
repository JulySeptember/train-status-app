import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Bell, BellOff, Plus, Send, Trash2 } from "lucide-react";

import { api } from "@/api";
import type { Alert, AlertDay, Railway } from "@/types";

import RailwayBadge from "@/components/RailwayBadge";
import { Button } from "@/components/ui/button";

import { useFavorites } from "@/lib/favorites";
import {
  needsHomeScreen,
  pushSupported,
  useAlerts,
  useDisablePush,
  useEnablePush,
  usePushConfig,
  usePushDevice,
} from "@/lib/push";
import { groupByOperator, useRailways } from "@/lib/railways";
import { cn } from "@/lib/utils";

const MAX_ALERTS = 10;

const DAYS: { value: AlertDay; label: string }[] = [
  { value: "weekday", label: "平日" },
  { value: "holiday", label: "土休日" },
];

// 00:00〜24:00 の30分ごと
const TIMES = Array.from({ length: 49 }, (_, i) => {
  const h = String(Math.floor(i / 2)).padStart(2, "0");
  return `${h}:${i % 2 === 0 ? "00" : "30"}`;
});

const DEFAULT_ALERT: Omit<Alert, "railwayId"> = {
  days: ["weekday"],
  start: "07:00",
  end: "09:30",
};

const selectClass =
  "h-8 rounded-lg border border-input bg-background px-2 text-sm dark:bg-input/30";

function ErrorText({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }
  return (
    <p role="alert" className="text-sm text-destructive-foreground">
      {error.message}
    </p>
  );
}

// この端末で受け取るかどうか
function DeviceSettings() {
  const device = usePushDevice();
  const enable = useEnablePush();
  const disable = useDisablePush();
  const test = useMutation({ mutationFn: api.sendTestNotification });

  if (!pushSupported()) {
    return (
      <p className="text-sm text-muted-foreground">
        このブラウザは通知に対応していません。
      </p>
    );
  }

  if (needsHomeScreen()) {
    return (
      <p className="text-sm text-muted-foreground">
        iPhone・iPad では、Safari
        の共有ボタンから「ホーム画面に追加」し、ホーム画面から開くと通知を受け取れます（iOS
        16.4 以降）。
      </p>
    );
  }

  const enabled = device.data === true;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        {enabled ? (
          <>
            <span className="flex items-center gap-1.5 text-sm text-brand">
              <Bell size={16} />
              この端末で通知を受け取ります
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={disable.isPending}
              onClick={() => disable.mutate()}
            >
              <BellOff />
              止める
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={test.isPending}
              onClick={() => test.mutate()}
            >
              <Send />
              テスト通知を送る
            </Button>
          </>
        ) : (
          <Button
            disabled={enable.isPending || device.isLoading}
            onClick={() => enable.mutate()}
          >
            <Bell />
            この端末で通知を受け取る
          </Button>
        )}
      </div>

      {test.isSuccess && (
        <p className="text-sm text-muted-foreground">
          {test.data.sent}台の端末に送りました。
        </p>
      )}
      <ErrorText error={enable.error ?? disable.error ?? test.error} />
    </div>
  );
}

function AlertEditor({
  initial,
  name,
  railway,
}: {
  initial: Alert;
  name: string;
  railway?: Railway;
}) {
  const client = useQueryClient();
  const [alert, setAlert] = useState(initial);

  const save = useMutation({
    mutationFn: () => api.setAlert(alert),
    onSuccess: () => client.invalidateQueries({ queryKey: ["alerts"] }),
  });
  const remove = useMutation({
    mutationFn: () => api.removeAlert(alert.railwayId),
    onSuccess: () => client.invalidateQueries({ queryKey: ["alerts"] }),
  });

  const changed = JSON.stringify(alert) !== JSON.stringify(initial);
  const valid =
    alert.days.length > 0 &&
    TIMES.indexOf(alert.start) < TIMES.indexOf(alert.end);

  const toggleDay = (day: AlertDay) =>
    setAlert({
      ...alert,
      days: alert.days.includes(day)
        ? alert.days.filter((d) => d !== day)
        : DAYS.map((d) => d.value).filter(
            (d) => d === day || alert.days.includes(d),
          ),
    });

  return (
    <li className="space-y-3 rounded-xl border bg-card p-4">
      <div className="flex items-center gap-2">
        <RailwayBadge railway={railway} className="size-6 text-xs" />
        <span className="flex-1 font-semibold">{name}</span>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`${name}の通知を消す`}
          disabled={remove.isPending}
          onClick={() => remove.mutate()}
        >
          <Trash2 />
        </Button>
      </div>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
        <div className="flex gap-3">
          {DAYS.map((d) => (
            <label key={d.value} className="flex items-center gap-1.5">
              <input
                type="checkbox"
                checked={alert.days.includes(d.value)}
                onChange={() => toggleDay(d.value)}
                className="size-4 accent-primary"
              />
              {d.label}
            </label>
          ))}
        </div>

        <div className="flex items-center gap-1.5">
          <select
            aria-label="開始時刻"
            value={alert.start}
            onChange={(e) => setAlert({ ...alert, start: e.target.value })}
            className={selectClass}
          >
            {TIMES.slice(0, -1).map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
          〜
          <select
            aria-label="終了時刻"
            value={alert.end}
            onChange={(e) => setAlert({ ...alert, end: e.target.value })}
            className={selectClass}
          >
            {TIMES.slice(1).map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </div>

        {changed && (
          <Button
            size="sm"
            disabled={!valid || save.isPending}
            onClick={() => save.mutate()}
          >
            保存
          </Button>
        )}
      </div>

      {!valid && (
        <p className="text-sm text-destructive-foreground">
          曜日を1つ以上選び、終了を開始より後にしてください。
        </p>
      )}
      <ErrorText error={save.error ?? remove.error} />
    </li>
  );
}

// 路線を選んで、通知の条件を足す。お気に入りの路線を先に並べる
function AddAlert({ exclude }: { exclude: string[] }) {
  const client = useQueryClient();
  const railways = useRailways();
  const { favorites } = useFavorites();
  const [railwayId, setRailwayId] = useState("");

  const add = useMutation({
    mutationFn: () => api.setAlert({ railwayId, ...DEFAULT_ALERT }),
    onSuccess: () => {
      setRailwayId("");
      client.invalidateQueries({ queryKey: ["alerts"] });
    },
  });

  const all = (railways.data ?? []).filter((r) => !exclude.includes(r.id));
  const favoriteIds = favorites?.railways ?? [];
  const favorite = all.filter((r) => favoriteIds.includes(r.id));
  // 同じ名前の路線（都営と西武の新宿線など）があるので、事業者ごとに分ける
  const groups = groupByOperator(all, (r) => r);

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2">
        <select
          aria-label="通知する路線"
          value={railwayId}
          onChange={(e) => setRailwayId(e.target.value)}
          className={cn(selectClass, "min-w-0 flex-1 sm:flex-none")}
        >
          <option value="">路線を選ぶ</option>
          {favorite.length > 0 && (
            <optgroup label="お気に入り">
              {favorite.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}（{r.operatorName}）
                </option>
              ))}
            </optgroup>
          )}
          {groups.map((g) => (
            <optgroup key={g.operator} label={g.name}>
              {g.items.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.name}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
        <Button
          variant="outline"
          size="sm"
          disabled={!railwayId || add.isPending}
          onClick={() => add.mutate()}
        >
          <Plus />
          追加
        </Button>
      </div>
      <ErrorText error={add.error} />
    </div>
  );
}

// 運行情報の通知の設定（アカウントの画面）
export default function NotificationSettings() {
  const config = usePushConfig();
  const alerts = useAlerts();
  const railways = useRailways();

  // 通知を使えないとき（VAPID の鍵が無い）は出さない
  if (!config.data) {
    return null;
  }

  const items = alerts.data ?? [];

  return (
    <section className="space-y-4">
      <div className="space-y-1">
        <h2 className="text-xl font-semibold">運行情報の通知</h2>
        <p className="text-sm text-muted-foreground">
          選んだ路線で、決めた曜日・時間帯に遅れ（10分以上）や運転見合わせが起きたとき、平常に戻ったときに知らせます。5分ごとに確かめます。
        </p>
        <p className="text-sm text-muted-foreground">
          曜日は運行日で決めます（0時台は前日の扱いで、土曜の0時台は「平日」。年末年始は「土休日」）。1時〜5時は確かめません。
        </p>
      </div>

      <DeviceSettings />

      {items.length > 0 && (
        <ul className="space-y-3">
          {items.map((a) => {
            const railway = railways.data?.find((r) => r.id === a.railwayId);
            return (
              <AlertEditor
                // 保存したら作り直して、編集前の値を新しくする
                key={JSON.stringify(a)}
                initial={a}
                name={railway?.name ?? a.railwayId}
                railway={railway}
              />
            );
          })}
        </ul>
      )}

      {items.length < MAX_ALERTS ? (
        <AddAlert exclude={items.map((a) => a.railwayId)} />
      ) : (
        <p className="text-sm text-muted-foreground">
          通知する路線は{MAX_ALERTS}路線まで登録できます。
        </p>
      )}
    </section>
  );
}
