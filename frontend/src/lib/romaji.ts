// 駅の選択で、ひらがな・カタカナ・ローマ字で駅を探すための変換。
// ODPT の駅データに読みは無いので、入力をヘボン式のローマ字に直し、駅の英語名と比べる

// ひらがな1文字のローマ字（ヘボン式）。小さい「ゃ」などは前の文字と合わせて読む（toRomaji）
// prettier-ignore
const KANA: Record<string, string> = {
  あ: "a", い: "i", う: "u", え: "e", お: "o",
  か: "ka", き: "ki", く: "ku", け: "ke", こ: "ko",
  が: "ga", ぎ: "gi", ぐ: "gu", げ: "ge", ご: "go",
  さ: "sa", し: "shi", す: "su", せ: "se", そ: "so",
  ざ: "za", じ: "ji", ず: "zu", ぜ: "ze", ぞ: "zo",
  た: "ta", ち: "chi", つ: "tsu", て: "te", と: "to",
  だ: "da", ぢ: "ji", づ: "zu", で: "de", ど: "do",
  な: "na", に: "ni", ぬ: "nu", ね: "ne", の: "no",
  は: "ha", ひ: "hi", ふ: "fu", へ: "he", ほ: "ho",
  ば: "ba", び: "bi", ぶ: "bu", べ: "be", ぼ: "bo",
  ぱ: "pa", ぴ: "pi", ぷ: "pu", ぺ: "pe", ぽ: "po",
  ま: "ma", み: "mi", む: "mu", め: "me", も: "mo",
  や: "ya", ゆ: "yu", よ: "yo",
  ら: "ra", り: "ri", る: "ru", れ: "re", ろ: "ro",
  わ: "wa", ゐ: "i", ゑ: "e", を: "o", ん: "n", ゔ: "vu",
  ぁ: "a", ぃ: "i", ぅ: "u", ぇ: "e", ぉ: "o",
  ゃ: "ya", ゅ: "yu", ょ: "yo", ゎ: "wa",
};

const SMALL_Y = new Set(["ゃ", "ゅ", "ょ"]);
const SMALL_VOWEL = new Set(["ぁ", "ぃ", "ぅ", "ぇ", "ぉ"]);

// かなをヘボン式のローマ字にする。かなでも英数字でもない文字（漢字など）があれば null
function toRomaji(text: string): string | null {
  // 半角カナを全角に、全角英数字を半角にしてから、カタカナをひらがなにする
  const hira = [...text.normalize("NFKC")].map((c) => {
    const code = c.charCodeAt(0);
    return code >= 0x30a1 && code <= 0x30f6
      ? String.fromCharCode(code - 0x60)
      : c;
  });

  let result = "";
  for (let i = 0; i < hira.length; i++) {
    const c = hira[i];
    const next = hira[i + 1];

    if (c === "っ") {
      // 次の音の子音を重ねる（ち・ちゃ の前は t）
      const following = next ? (KANA[next] ?? "") : "";
      result += following.startsWith("ch") ? "t" : following.charAt(0);
      continue;
    }

    const base = KANA[c];
    if (base === undefined) {
      // 長音の記号・空白・記号は比べるときに除く。ASCII はそのまま使う
      if (c === "ー" || /^[\x20-\x7e]$/.test(c) || /\s/.test(c)) {
        result += c;
        continue;
      }
      return null;
    }

    if (next && SMALL_Y.has(next) && base.endsWith("i") && base.length > 1) {
      // きゃ → kya、しゃ → sha、じゃ → ja
      const head = base.slice(0, -1);
      const vowel = KANA[next].slice(1);
      result += /^(sh|ch|j)$/.test(head) ? head + vowel : head + "y" + vowel;
      i++;
      continue;
    }

    if (next && SMALL_VOWEL.has(next)) {
      // ふぁ → fa、てぃ → ti、うぃ → wi
      const head = base.length > 1 ? base.slice(0, -1) : "w";
      result += head + KANA[next];
      i++;
      continue;
    }

    result += base;
  }

  return result;
}

// ローマ字を比べる形にそろえる。長音の記号（ō）と長音の書き方（ou・oo・uu）を除き、
// b・p・m の前の m を n にする（Jimbocho と「じんぼうちょう」を同じにする）。英数字以外は除く
export function romajiKey(romaji: string): string {
  return romaji
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]/g, "")
    .replace(/m(?=[bpm])/g, "n")
    .replace(/o[ou]+/g, "o")
    .replace(/uu+/g, "u");
}

// 検索語を駅の英語名と比べる形にする。漢字を含むとき・比べる文字が無いときは null
export function searchKey(keyword: string): string | null {
  const romaji = toRomaji(keyword);
  if (romaji === null) return null;
  const key = romajiKey(romaji);
  return key === "" ? null : key;
}
