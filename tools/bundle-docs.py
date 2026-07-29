#!/usr/bin/env python3
"""ビルド済みの教材サイトを、HTML 1枚にまとめる。

スマホへAirDropして、電波が無いところでも読めるようにするための道具です。

    cd docs && npm run build
    python3 tools/bundle-docs.py

出来上がりは 1ファイルだけ。CSSも画像もスクリプトも中に入っているので、
どこへ置いても、どの端末で開いても同じように表示されます。

file:// で開くとESモジュールが読み込めないため、
クイズの動作だけはこのスクリプトが素のJSで書き直して埋め込みます。
"""

import html
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
DIST = ROOT / "docs" / "dist"
OUT = ROOT / "shortlink-15days.html"
BASE = "/study-backend-scale"


def extract_block(source: str, opening: re.Pattern) -> str:
    """開始タグを見つけて、対応する閉じタグまでを切り出す。

    div が入れ子になっているので、単純な正規表現では終端を誤る。
    開いた数と閉じた数を数えて対応を取る。
    """
    match = opening.search(source)
    if not match:
        return ""
    start = match.start()
    depth = 0
    for token in re.finditer(r"<(/?)div\b[^>]*?(/?)>", source[start:]):
        if token.group(2) == "/":  # 自己終端 <div/> は数えない
            continue
        depth += -1 if token.group(1) else 1
        if depth == 0:
            return source[start : start + token.end()]
    return source[start:]


def page_parts(path: pathlib.Path) -> tuple[str, str, list[str]]:
    source = path.read_text()

    title_match = re.search(r"<h1\b[^>]*>(.*?)</h1>", source, re.S)
    title = re.sub(r"<[^>]+>", "", title_match.group(1)).strip() if title_match else path.parent.name

    body = extract_block(source, re.compile(r'<div class="sl-markdown-content"'))
    # Expressive Code が本文へ差し込む外部参照を落とす。
    # CSS はこのあと丸ごと埋め込むので不要、JS はコードブロックの装飾用で必須ではない。
    body = re.sub(r'<link[^>]*rel="stylesheet"[^>]*>', "", body)
    body = re.sub(r'<script[^>]*\bsrc="[^"]*"[^>]*>\s*</script>', "", body)
    # インラインの module を素の script にする。
    # file:// で開いたときブラウザが module を実行しないことがあるため。
    # 中身に import は無く、data-initialized の番人があるので二重初期化もしない。
    body = body.replace('<script type="module">', "<script>")

    styles = re.findall(r"<style>(.*?)</style>", source, re.S)
    return title, body, styles


def localize(body: str, slug: str) -> str:
    """ページ間リンクをファイル内アンカーへ、IDを日ごとに一意にする。"""
    # 別ページへのリンク → そのセクションへ
    body = re.sub(rf'href="{re.escape(BASE)}/(day\d\d)/?"', r'href="#\1"', body)
    body = re.sub(rf'href="{re.escape(BASE)}/?"', 'href="#top"', body)

    # 見出しIDが日をまたいで重複するので、slugを前置きして一意にする
    body = re.sub(r'id="([^"]+)"', lambda m: f'id="{slug}--{m.group(1)}"', body)
    body = re.sub(r'href="#([^"]+)"', lambda m: f'href="#{slug}--{m.group(1)}"', body)
    # 上で自分自身のセクションリンクまで書き換わるので戻す
    body = body.replace(f'href="#{slug}--day', 'href="#day')
    return body


RUNTIME_JS = """
// 端末の設定に合わせて配色を決める（Starlightのテーマ変数をそのまま使う）。
// クイズの動作は Astro が本文に埋めたスクリプトがそのまま担当する。
(function () {
  var dark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)');
  var apply = function (on) {
    document.documentElement.dataset.theme = on ? 'dark' : 'light';
  };
  apply(dark && dark.matches);
  if (dark && dark.addEventListener) {
    dark.addEventListener('change', function (e) { apply(e.matches); });
  }
})();
"""

EXTRA_CSS = """
/* 1枚にまとめたとき用の最小限の調整 */
body { margin: 0; }
.offline-wrap { max-width: 52rem; margin: 0 auto; padding: 1rem 1rem 6rem; }
.offline-toc { margin: 0 0 3rem; }
.offline-toc ol { padding-left: 1.2rem; line-height: 2; }
.offline-day { padding-top: 2rem; margin-top: 3rem; border-top: 1px solid var(--sl-color-gray-5, #cbd5e1); }
.offline-day > h1 { margin-top: 0; }
.offline-top {
  position: fixed; right: 1rem; bottom: 1rem; z-index: 10;
  padding: .6rem .9rem; border-radius: 999px; text-decoration: none;
  background: var(--sl-color-accent, #2563eb); color: #fff; font-size: .85rem;
  box-shadow: 0 2px 12px rgba(0,0,0,.25);
}
/* サイト側のコピーボタンはオフラインでは意味が薄いので隠す */
.expressive-code .copy { display: none; }
"""


def main() -> int:
    if not DIST.is_dir():
        print("docs/dist がありません。先に `cd docs && npm run build` を実行してください。", file=sys.stderr)
        return 1

    pages = [("index", DIST / "index.html")]
    for n in range(1, 16):
        slug = f"day{n:02d}"
        path = DIST / slug / "index.html"
        if path.is_file():
            pages.append((slug, path))

    css = "\n".join((DIST / "_astro" / name).read_text() for name in sorted(
        p.name for p in (DIST / "_astro").glob("*.css")))

    seen_styles: list[str] = []
    sections: list[str] = []
    toc: list[str] = []

    for slug, path in pages:
        title, body, styles = page_parts(path)
        if not body:
            print(f"  警告: {slug} の本文を取り出せませんでした", file=sys.stderr)
            continue
        for style in styles:
            if style not in seen_styles:
                seen_styles.append(style)

        # 目次のnavが id="top" を使うので、概要ページは別のIDにする
        anchor = "overview" if slug == "index" else slug
        label = "コース概要" if slug == "index" else title
        toc.append(f'<li><a href="#{anchor}">{html.escape(label)}</a></li>')
        sections.append(
            f'<section class="offline-day" id="{anchor}">'
            f"<h1>{html.escape(label)}</h1>\n{localize(body, slug)}</section>"
        )
        print(f"  取り込み: {slug}  {len(body) // 1024}KB  {title}")

    document = f"""<!doctype html>
<html lang="ja" data-theme="light">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>URL短縮で学ぶバックエンドスケーラビリティ — 全15日</title>
<style>{css}</style>
<style>{"".join(seen_styles)}</style>
<style>{EXTRA_CSS}</style>
</head>
<body>
<div class="offline-wrap sl-markdown-content">
<nav class="offline-toc" id="top">
<h1>URL短縮で学ぶバックエンドスケーラビリティ</h1>
<p>全15日ぶんをこの1ファイルに入れてあります。通信がなくても読めます。</p>
<ol>{"".join(toc)}</ol>
</nav>
{"".join(sections)}
</div>
<a class="offline-top" href="#top">↑ 目次</a>
<script>{RUNTIME_JS}</script>
</body>
</html>
"""

    OUT.write_text(document)
    size = OUT.stat().st_size
    print(f"\n{OUT.relative_to(ROOT)} を作りました（{size / 1024 / 1024:.1f}MB、{len(sections)}ページ）")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
