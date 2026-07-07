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
  trainNumber: string;
  destination: string;
}

export interface Passenger {
  year: number;
  count: number;
}

export interface DirectionTimetable {
  calendar: string;
  railDirection: string;
  timetables: Timetable[];
}

export interface StationDetail {
  id: string;
  name: string;
  timetables: DirectionTimetable[];
  passengers: Passenger[];
}

export interface TrainLocation {
  trainNumber: string;
  railway: string;
  fromStation: string;
  toStation: string;
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
