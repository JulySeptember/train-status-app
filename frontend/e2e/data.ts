// API の応答の固定データ。src/types.ts の型を付け、DTO を変えたら tsc で気づけるようにする
import type {
  JourneySearch,
  Railway,
  Station,
  StationDetail,
  StationSummary,
  Timetable,
  TrainLocation,
  TrainStatus,
} from "../src/types.ts";

const TOEI = { operator: "odpt.Operator:Toei", operatorName: "都営交通" };

export const ASAKUSA = "odpt.Railway:Toei.Asakusa";
export const MITA = "odpt.Railway:Toei.Mita";
export const OEDO = "odpt.Railway:Toei.Oedo";

const NORTHBOUND = "odpt.RailDirection:Northbound";
const SOUTHBOUND = "odpt.RailDirection:Southbound";

export const railways: Railway[] = [
  {
    id: ASAKUSA,
    name: "浅草線",
    lineCode: "A",
    color: "#E85298",
    ...TOEI,
    directions: [
      { id: NORTHBOUND, name: "押上方面" },
      { id: SOUTHBOUND, name: "西馬込方面" },
    ],
  },
  {
    id: MITA,
    name: "三田線",
    lineCode: "I",
    color: "#0079C2",
    ...TOEI,
    directions: [
      { id: NORTHBOUND, name: "西高島平方面" },
      { id: SOUTHBOUND, name: "目黒方面" },
    ],
  },
  {
    id: OEDO,
    name: "大江戸線",
    lineCode: "E",
    color: "#B6007A",
    ...TOEI,
    directions: [
      { id: "odpt.RailDirection:OuterLoop", name: "都庁前方面（外回り）" },
      { id: "odpt.RailDirection:InnerLoop", name: "都庁前方面（内回り）" },
    ],
  },
];

// 浅草線・大江戸線は平常（「１５分以上の遅延はありません」は全角の数字で配信される）、三田線は遅れ
export const statuses: TrainStatus[] = [
  {
    railwayId: ASAKUSA,
    railway: "浅草線",
    status: "現在、平常どおり運転しています。",
  },
  {
    railwayId: MITA,
    railway: "三田線",
    status: "人身事故の影響で、三田線の一部の列車に遅れが出ています。",
  },
  {
    railwayId: OEDO,
    railway: "大江戸線",
    status: "現在、１５分以上の遅延はありません。",
  },
];

const asakusaStation = (key: string) => `odpt.Station:Toei.Asakusa.${key}`;

export const GOTANDA = asakusaStation("Gotanda");
export const TOGOSHI = asakusaStation("Togoshi");
export const NISHI_MAGOME = asakusaStation("NishiMagome");
export const ASAKUSA_DAIMON = asakusaStation("Daimon");
export const OEDO_DAIMON = "odpt.Station:Toei.Oedo.Daimon";
export const ROPPONGI = "odpt.Station:Toei.Oedo.Roppongi";

// 浅草線の駅（西馬込から路線の順）
export const asakusaStations: Station[] = [
  { id: NISHI_MAGOME, name: "西馬込" },
  { id: asakusaStation("Magome"), name: "馬込" },
  { id: asakusaStation("Nakanobu"), name: "中延" },
  { id: TOGOSHI, name: "戸越" },
  { id: GOTANDA, name: "五反田" },
  { id: asakusaStation("Takanawadai"), name: "高輪台" },
  { id: asakusaStation("Sengakuji"), name: "泉岳寺" },
  { id: asakusaStation("Mita"), name: "三田" },
  { id: ASAKUSA_DAIMON, name: "大門" },
];

// 浅草線の駅の座標（西馬込から路線の順）
const asakusaCoords: [number, number][] = [
  [35.5866, 139.7058],
  [35.5964, 139.7116],
  [35.6057, 139.7128],
  [35.6146, 139.7163],
  [35.6261, 139.7234],
  [35.6317, 139.7301],
  [35.6389, 139.7398],
  [35.6481, 139.7486],
  [35.6566, 139.7548],
];

// 全駅の一覧。浅草線と大江戸線の大門は、経路検索では1つの駅（代表は浅草線の大門）
export const allStations: StationSummary[] = [
  ...asakusaStations.map((s, i) => ({
    ...s,
    railwayId: ASAKUSA,
    journeyStation: s.id,
    lat: asakusaCoords[i][0],
    lon: asakusaCoords[i][1],
  })),
  {
    id: OEDO_DAIMON,
    name: "大門",
    railwayId: OEDO,
    journeyStation: ASAKUSA_DAIMON,
    lat: 35.6565,
    lon: 139.7559,
  },
  {
    id: ROPPONGI,
    name: "六本木",
    railwayId: OEDO,
    journeyStation: ROPPONGI,
    lat: 35.664,
    lon: 139.7323,
  },
];

export const RUNNING_TRAIN = "odpt.Train:Toei.Asakusa.803T";
export const WAITING_TRAIN = "odpt.Train:Toei.Asakusa.815T";

function departure(
  time: string,
  trainId: string,
  destination: string,
  trainType = "普通",
): Timetable {
  return {
    time,
    trainId,
    trainNumber: trainId.split(".").at(-1) ?? "",
    destination,
    trainTypeId: `odpt.TrainType:Toei.${trainType === "普通" ? "Local" : "Express"}`,
    trainType,
  };
}

// 五反田の時刻表。本日（NOW）は平日で、08:00 の次発は押上方面が 08:03、西馬込方面が 08:05
export const gotanda: StationDetail = {
  id: GOTANDA,
  name: "五反田",
  trainLocationAvailable: true,
  timetables: [
    {
      calendar: "odpt.Calendar:Weekday",
      railDirection: NORTHBOUND,
      isToday: true,
      timetables: [
        departure("07:52", "odpt.Train:Toei.Asakusa.751T", "押上"),
        departure("08:03", RUNNING_TRAIN, "押上"),
        departure("08:10", "odpt.Train:Toei.Asakusa.809T", "成田空港", "快特"),
      ],
    },
    {
      calendar: "odpt.Calendar:Weekday",
      railDirection: SOUTHBOUND,
      isToday: true,
      timetables: [
        departure("07:58", "odpt.Train:Toei.Asakusa.756T", "西馬込"),
        departure("08:05", "odpt.Train:Toei.Asakusa.802T", "西馬込"),
      ],
    },
    {
      calendar: "odpt.Calendar:SaturdayHoliday",
      railDirection: NORTHBOUND,
      isToday: false,
      timetables: [departure("08:06", "odpt.Train:Toei.Asakusa.805T", "押上")],
    },
    {
      calendar: "odpt.Calendar:SaturdayHoliday",
      railDirection: SOUTHBOUND,
      isToday: false,
      timetables: [
        departure("08:12", "odpt.Train:Toei.Asakusa.810T", "西馬込"),
      ],
    },
  ],
};

const trainBase = {
  trainNumber: "803T",
  railwayId: ASAKUSA,
  railway: "浅草線",
  trainTypeId: "odpt.TrainType:Toei.Local",
  trainType: "普通",
  railDirection: NORTHBOUND,
  destination: "押上",
  updatedAt: "2026-10-19T07:59:45+09:00",
};

// 戸越 → 五反田を2分遅れで走っている列車
export const runningTrain: TrainLocation = {
  ...trainBase,
  trainId: RUNNING_TRAIN,
  fromStationId: TOGOSHI,
  fromStation: "戸越",
  toStationId: GOTANDA,
  toStation: "五反田",
  stopped: false,
  delay: 120,
  delayAvailable: true,
  available: true,
  message: "",
};

// 始発駅で発車を待っている列車（odpt:Train に位置が無い）
export const waitingTrain: TrainLocation = {
  ...trainBase,
  trainId: WAITING_TRAIN,
  trainNumber: "815T",
  fromStationId: "",
  fromStation: "",
  toStationId: "",
  toStation: "",
  stopped: false,
  delay: 0,
  delayAvailable: false,
  available: false,
  message: "西馬込を08:15に出発する予定です。出発すると位置を表示します",
  notRunning: "beforeDeparture",
  scheduledStationId: NISHI_MAGOME,
  scheduledStation: "西馬込",
  scheduledTime: "08:15",
};

// 五反田 → 六本木。浅草線で大門まで行き（2分遅れ）、大江戸線に乗り換える
export const journeySearch: JourneySearch = {
  delayApplied: true,
  suspendedRailways: [],
  journeys: [
    {
      departureTime: "08:05",
      arrivalTime: "08:26",
      transfers: 1,
      walkBeforeMinutes: 0,
      walkAfterMinutes: 0,
      legs: [
        {
          railway: ASAKUSA,
          railwayName: "浅草線",
          train: RUNNING_TRAIN,
          trainNumber: "803T",
          trainType: "odpt.TrainType:Toei.Local",
          trainTypeName: "普通",
          destination: "odpt.Station:Toei.Asakusa.Oshiage",
          destinationName: "押上",
          from: GOTANDA,
          fromName: "五反田",
          to: ASAKUSA_DAIMON,
          toName: "大門",
          departureTime: "08:05",
          arrivalTime: "08:13",
          delayMinutes: 2,
          through: false,
          estimated: false,
        },
        {
          railway: OEDO,
          railwayName: "大江戸線",
          train: "odpt.Train:Toei.Oedo.1810",
          trainNumber: "1810",
          trainType: "odpt.TrainType:Toei.Local",
          trainTypeName: "普通",
          destination: "odpt.Station:Toei.Oedo.Tochomae",
          destinationName: "都庁前",
          from: OEDO_DAIMON,
          fromName: "大門",
          to: ROPPONGI,
          toName: "六本木",
          departureTime: "08:18",
          arrivalTime: "08:26",
          delayMinutes: 0,
          through: false,
          estimated: false,
        },
      ],
    },
  ],
};
