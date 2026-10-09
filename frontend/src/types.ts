export interface TrainStatus {
  railwayId: string;
  railway: string;
  status: string;
  // 事業者の運行情報を取得できなかったときは true（status は「運行情報を取得できませんでした」）
  unavailable?: boolean;
}

export interface Railway {
  id: string;
  name: string;

  // GET /api/routes だけが返す。ODPT が配信していない路線では無い
  lineCode?: string;
  color?: string;
}

export interface Station {
  id: string;
  name: string;
}

export interface Timetable {
  time: string;
  trainId: string;
  trainNumber: string;
  destination: string;
  trainTypeId: string;
  trainType: string;
}

export interface Passenger {
  year: number;
  count: number;
}

export interface DirectionTimetable {
  calendar: string;
  railDirection: string;
  timetables: Timetable[];
  isToday: boolean;
}

export interface StationDetail {
  id: string;
  name: string;
  trainLocationAvailable: boolean;
  timetables: DirectionTimetable[];
  passengers: Passenger[];
}

export interface TrainLocation {
  trainId: string;
  trainNumber: string;
  railwayId: string;
  railway: string;
  trainTypeId: string;
  trainType: string;
  railDirection: string;
  destination: string;
  fromStationId: string;
  fromStation: string;
  toStationId: string;
  toStation: string;
  stopped: boolean;
  delay: number;
  delayAvailable: boolean;
  updatedAt: string;
  available: boolean;
  message: string;
  // available が false のとき、本日のダイヤから見た状態。本日のダイヤに無い列車では無い
  notRunning?: "beforeDeparture" | "finished" | "noData";
  scheduledStationId?: string;
  scheduledStation?: string;
  scheduledTime?: string;
}

export interface Fare {
  from: string;
  to: string;
  icFare: number;
  ticketFare: number;
}

export interface JourneyLeg {
  railway: string;
  railwayName: string;
  train: string;
  trainNumber: string;
  trainType: string;
  trainTypeName: string;
  // 大江戸線の環状部などでは行先が無く、空になる
  destination: string;
  destinationName: string;
  from: string;
  fromName: string;
  to: string;
  toName: string;
  // 遅れを足した時刻。遅れは路線・方向ごとの見込みで、現在から1時間以内の時刻にだけ足す
  departureTime: string;
  arrivalTime: string;
  // 乗る駅での発車の遅れ（分）
  delayMinutes: number;
}

export interface Journey {
  departureTime: string;
  arrivalTime: string;
  transfers: number;
  legs: JourneyLeg[];
}

export interface JourneySearch {
  journeys: Journey[];
  // 運行状況を取得できなかったときは false で、時刻表どおりの結果になる
  delayApplied: boolean;
  // 運転を見合わせているため使わなかった路線
  suspendedRailways: Railway[];
}

// departAt と arriveBy（HH:MM）はどちらか一方だけ。どちらも無ければ現在時刻に出発する
export interface JourneyQuery {
  from: string;
  to: string;
  departAt?: string;
  arriveBy?: string;
  maxTransfers?: number;
  avoid?: string[];
  // false なら遅延・運転見合わせを反映せず、時刻表どおりに探す（省略時は反映する）
  realtime?: boolean;
}

// AI エージェント（POST /api/chat）。会話の履歴はブラウザが持ち、毎回送る
export interface ChatMessage {
  role: "user" | "assistant";
  text: string;
}

// AI が呼んだ道具と、画面向けの短い説明
export interface ChatStep {
  tool: string;
  label: string;
}

export interface ChatResponse {
  reply: string;
  steps: ChatStep[];
  // 最後に探した経路。時刻などは AI の文章ではなく、これをそのまま表示する
  journeys?: JourneySearch;
}

// quota_exceeded・rate_limited・unavailable などで表示を分ける
export interface ChatErrorBody {
  error: string;
  code: string;
}
