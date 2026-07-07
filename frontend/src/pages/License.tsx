export default function License() {
  return (
    <div className="prose max-w-4xl">
      <h1>データ提供・ライセンス</h1>

      <p>
        Train Status App
        は東京都交通局が公開するオープンデータを加工して利用しています。
      </p>

      <h2>データ提供者</h2>

      <p>東京都交通局・公共交通オープンデータ協議会</p>

      <h2>利用データ</h2>

      <ul>
        <li>運行情報（GTFS Realtime Alert）</li>
        <li>列車位置情報（GTFS Realtime VehiclePosition）</li>
        <li>列車運行情報（GTFS Realtime TripUpdate）</li>
        <li>路線情報</li>
        <li>駅情報</li>
        <li>駅時刻表</li>
        <li>列車時刻表</li>
        <li>運賃情報</li>
        <li>乗降者数情報</li>
      </ul>

      <h2>ライセンス</h2>

      <p>
        このアプリは東京都交通局・公共交通オープンデータ協議会が提供するオープンデータを改変して利用しています。
      </p>

      <p>
        ライセンス：
        <a
          href="https://creativecommons.org/licenses/by/4.0/deed.ja"
          target="_blank"
          rel="noreferrer"
        >
          Creative Commons Attribution 4.0 International (CC BY 4.0)
        </a>
      </p>
    </div>
  );
}
