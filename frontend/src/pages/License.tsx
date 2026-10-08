export default function License() {
  return (
    <div className="prose max-w-4xl">
      <h1>データ提供・ライセンス</h1>

      <p>
        このページは東京都交通局が公開するオープンデータを加工して利用しています。
      </p>
      <br></br>
      <h2>データ提供者</h2>

      <p>東京都交通局・公共交通オープンデータ協議会</p>
      <br></br>

      <h2>利用データ</h2>

      <ul>
        <li>列車位置情報</li>
        <li>列車運行情報</li>
        <li>路線情報</li>
        <li>駅情報</li>
        <li>駅時刻表</li>
        <li>列車時刻表</li>
        <li>運賃情報</li>
        <li>乗降者数情報</li>
      </ul>
      <br></br>

      <h2>AI の利用について</h2>

      <p>
        「AI に聞く」では、Google の Gemini API を使っています。入力内容は AI
        の提供元（Google）に送信され、サービス改善に利用されることがあります。個人情報は入力しないでください。
      </p>

      <p>
        <a
          href="https://ai.google.dev/gemini-api/terms"
          target="_blank"
          rel="noreferrer"
        >
          Gemini API 利用規約
        </a>
      </p>

      <p>
        AI
        の回答は誤ることがあります。駅・時刻・経路・運行状況は、アプリが持つオープンデータから取得しています。
      </p>
      <br></br>

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
