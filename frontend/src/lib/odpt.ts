export function directionLabel(value: string) {
  switch (value) {
    case "odpt.RailDirection:Southbound":
      return "南行";

    case "odpt.RailDirection:Northbound":
      return "北行";

    case "odpt.RailDirection:Eastbound":
      return "東行";

    case "odpt.RailDirection:Westbound":
      return "西行";

    case "odpt.RailDirection:InnerLoop":
      return "内回り";

    case "odpt.RailDirection:OuterLoop":
      return "外回り";

    case "odpt.RailDirection:Inbound":
      return "上り";

    case "odpt.RailDirection:Outbound":
      return "下り";

    case "odpt.RailDirection:Toei.Waseda":
      return "早稲田方面";

    case "odpt.RailDirection:Toei.Minowabashi":
      return "三ノ輪橋方面";

    default:
      return value.replace("odpt.RailDirection:", "");
  }
}

export function calendarLabel(value: string) {
  switch (value) {
    case "odpt.Calendar:Weekday":
      return "平日";

    case "odpt.Calendar:SaturdayHoliday":
      return "土休日";

    default:
      return value;
  }
}
