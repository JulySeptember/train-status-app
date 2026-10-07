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
