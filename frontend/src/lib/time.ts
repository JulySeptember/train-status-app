// 運行日は 3時で切り替わる（バックエンドの internal/calendar と同じ規則）
const SERVICE_DAY_START_HOUR = 3;

const tokyoTime = new Intl.DateTimeFormat("en-US", {
  timeZone: "Asia/Tokyo",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

// 運行日の 0時からの経過分。3時前は前日の運行日の続きとして +24時間する
function serviceMinutes(hour: number, minute: number): number {
  const h = hour < SERVICE_DAY_START_HOUR ? hour + 24 : hour;
  return h * 60 + minute;
}

// 現在時刻（日本時間）を運行日の経過分で返す。端末のタイムゾーンには依存しない
export function nowServiceMinutes(now: Date = new Date()): number {
  const parts = tokyoTime.formatToParts(now);
  const get = (type: string) =>
    Number(parts.find((p) => p.type === type)?.value ?? 0);

  return serviceMinutes(get("hour"), get("minute"));
}

// 時刻表の "HH:MM" を運行日の経過分にする。読めないときは null
export function timeToServiceMinutes(time: string): number | null {
  const m = /^(\d{1,2}):(\d{2})$/.exec(time);
  if (!m) {
    return null;
  }

  return serviceMinutes(Number(m[1]), Number(m[2]));
}
