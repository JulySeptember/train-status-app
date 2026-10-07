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
  departureTime: string;
  arrivalTime: string;
}

export interface Journey {
  departureTime: string;
  arrivalTime: string;
  transfers: number;
  legs: JourneyLeg[];
}

export interface JourneySearch {
  journeys: Journey[];
}

// departAt と arriveBy（HH:MM）はどちらか一方だけ。どちらも無ければ現在時刻に出発する
export interface JourneyQuery {
  from: string;
  to: string;
  departAt?: string;
  arriveBy?: string;
  maxTransfers?: number;
  avoid?: string[];
}
