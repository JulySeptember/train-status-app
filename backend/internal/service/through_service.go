package service

// throughServiceStations は、直通運転先（都営以外）の行先駅の駅名。
//
// 都営の駅時刻表の行先（odpt:destinationStation）には他社の駅が含まれるが、
// 他社の駅データは ODPT の公開 API（api-public.odpt.org）からは取得できない。
// そのため、station_timetable.json に現れる他社の行先駅をここに列挙している。
//
// 他社のデータ（assets/extra）を読み込むときは、他社の行先駅の駅名は extra/destination_station.json から引く。
// ここには、ODPT にデータの無い事業者（京成・北総・相鉄など）の駅を書く。
//
// 更新方法: assets を更新したあとにサーバーを起動し、
// 「unknown destination station」の警告が出た駅を追加する。
var throughServiceStations = map[string]string{
	// 会津鉄道
	"odpt.Station:Aizu.Aizu.AizuTajima": "会津田島",

	// 富士山麓電気鉄道
	"odpt.Station:Fujikyu.Fujikyu.Kawaguchiko": "河口湖",

	// 伊豆箱根鉄道・伊豆急行
	"odpt.Station:IzuHakone.Sunzu.Shuzenji":    "修善寺",
	"odpt.Station:Izukyu.Izukyu.IzukyuShimoda": "伊豆急下田",

	// JR東海・JR西日本・JR四国（JR東日本の直通・寝台特急の行先）
	"odpt.Station:JR-Central.Gotemba.Gotemba":      "御殿場",
	"odpt.Station:JR-Central.Tokaido.Numazu":       "沼津",
	"odpt.Station:JR-West.Sanin.Izumoshi":          "出雲市",
	"odpt.Station:JR-Shikoku.SetoOhashi.Takamatsu": "高松",

	// 横浜高速鉄道みなとみらい線
	"odpt.Station:Minatomirai.Minatomirai.MotomachiChukagai": "元町・中華街",

	// 箱根登山鉄道
	"odpt.Station:OdakyuHakone.HakoneTozan.HakoneYumoto": "箱根湯本",

	// 埼玉高速鉄道
	"odpt.Station:SaitamaRailway.SaitamaRailway.Hatogaya":    "鳩ヶ谷",
	"odpt.Station:SaitamaRailway.SaitamaRailway.UrawaMisono": "浦和美園",

	// 東葉高速鉄道
	"odpt.Station:ToyoRapid.ToyoRapid.ToyoKatsutadai":     "東葉勝田台",
	"odpt.Station:ToyoRapid.ToyoRapid.YachiyoMidorigaoka": "八千代緑が丘",

	// 北総線
	"odpt.Station:Hokuso.Hokuso.ImbaNihonIdai":   "印旛日本医大",
	"odpt.Station:Hokuso.Hokuso.InzaiMakinohara": "印西牧の原",

	// 京急線
	"odpt.Station:Keikyu.Airport.HanedaAirportTerminal1and2": "羽田空港第1・第2ターミナル",
	"odpt.Station:Keikyu.Kurihama.KeikyuKurihama":            "京急久里浜",
	"odpt.Station:Keikyu.Kurihama.Misakiguchi":               "三崎口",
	"odpt.Station:Keikyu.Kurihama.Miurakaigan":               "三浦海岸",
	"odpt.Station:Keikyu.Main.KanagawaShimmachi":             "神奈川新町",
	"odpt.Station:Keikyu.Main.KanazawaBunko":                 "金沢文庫",
	"odpt.Station:Keikyu.Main.Shinagawa":                     "品川",
	"odpt.Station:Keikyu.Zushi.ZushiHayama":                  "逗子・葉山",

	// 京王線
	"odpt.Station:Keio.KeioNew.Sasazuka":          "笹塚",
	"odpt.Station:Keio.Sagamihara.Hashimoto":      "橋本",
	"odpt.Station:Keio.Sagamihara.KeioTamaCenter": "京王多摩センター",
	"odpt.Station:Keio.Sagamihara.Wakabadai":      "若葉台",

	// 京成線
	"odpt.Station:Keisei.Main.KeiseiNarita":                      "京成成田",
	"odpt.Station:Keisei.Main.KeiseiSakura":                      "京成佐倉",
	"odpt.Station:Keisei.Main.KeiseiTakasago":                    "京成高砂",
	"odpt.Station:Keisei.Main.NaritaAirportTerminal1":            "成田空港",
	"odpt.Station:Keisei.Main.Sogosando":                         "宗吾参道",
	"odpt.Station:Keisei.NaritaSkyAccess.NaritaAirportTerminal1": "成田空港",
	"odpt.Station:Keisei.Oshiage.Aoto":                           "青砥",

	// 芝山鉄道線
	"odpt.Station:Shibayama.Shibayama.ShibayamaChiyoda": "芝山千代田",

	// 相鉄線
	"odpt.Station:Sotetsu.Izumino.Shonandai":           "湘南台",
	"odpt.Station:Sotetsu.Main.Ebina":                  "海老名",
	"odpt.Station:Sotetsu.Main.Yamato":                 "大和",
	"odpt.Station:Sotetsu.SotetsuShinYokohama.Nishiya": "西谷",

	// 東急線
	"odpt.Station:Tokyu.Meguro.Hiyoshi":                 "日吉",
	"odpt.Station:Tokyu.Meguro.MusashiKosugi":           "武蔵小杉",
	"odpt.Station:Tokyu.TokyuShinYokohama.ShinYokohama": "新横浜",
}
