// SPA のルーティング用に、静的ファイル以外のリクエストを /index.html に書き換える。
// 画面の URL には odpt.Station:Toei.Asakusa.Asakusa のように "." が入るので、
// 拡張子の有無ではなく「/assets/ 以下」と「トップレベルのファイル」を静的ファイルとみなす。
function handler(event) {
  var request = event.request;
  var uri = request.uri;

  if (uri.startsWith("/assets/") || /^\/[^/]+\.[a-z0-9]+$/.test(uri)) {
    return request;
  }

  request.uri = "/index.html";
  return request;
}
