export interface TrainStatus {
  railway: string;
  status: string;
}

export interface Railway {
  id: string;
  name: string;
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
  railway: string;
  fromStation: string;
  toStation: string;
  stopped: boolean;
  delay: number;
  available: boolean;
  message: string;
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
