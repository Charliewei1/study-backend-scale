# HARNESS.md — ワーカー共通規約(全 Codex ワーカー必読)

このリポジトリは「URL短縮サービスをテーマに Go / Docker / Kubernetes / スケーラビリティを15日で学ぶ」教材リポジトリ。
各ワーカーは **割り当てられたタスクの範囲のファイルだけ** を作成・編集すること。範囲外のファイルには触れない。

## リポジトリ構成
```
app/       # Go 製 URL 短縮サービス(日ごとに進化する)
deploy/    # Dockerfile, compose.yaml, k8s/ マニフェスト
load/      # k6 負荷試験スクリプト (dayNN-*.js)
docs/      # Astro Starlight 製の教材サイト(GitHub Pages で公開)
plans/     # オーケストレーターのプラン(編集禁止・参照のみ)
```

## Go アプリ規約 (app/)
- module 名: `github.com/study-backend-scale/shortlink`
- Go 1.24+, 標準ライブラリ優先。ルーティングは `net/http` の `http.ServeMux`(Go 1.22 のパターンマッチ)を使う。フレームワーク(gin等)は使わない(教材として標準を見せるため)。
- レイアウト: `cmd/server/main.go`, `internal/handler/`, `internal/storage/`, `internal/shortener/` を基本とし、日ごとの指示に従い拡張。
- API 契約(全日程で不変):
  - `POST /api/links` body `{"url":"https://..."}` → 201 `{"code":"<base62>","short_url":"http://localhost:8080/<code>"}`
  - `GET /{code}` → 302 リダイレクト、未知コードは 404
  - `GET /healthz` → 200 `ok`
- ポートは環境変数 `PORT`(デフォルト 8080)。設定はすべて環境変数。
- エラーハンドリングは Go 慣用(`fmt.Errorf("...: %w", err)`)。テストは table-driven + `httptest`。
- 完了条件: `go build ./...`, `go vet ./...`, `go test ./...` が全部通ること(ワーカー自身が実行して確認する)。

## 教材サイト規約 (docs/)
- Astro + Starlight。教材本文は `docs/src/content/docs/` 配下の `.mdx`。
- 言語は **日本語**。授業のイメージ: 講師が語りかける丁寧な解説。「なぜそうするのか」を必ず説明する。
- 各日のページ構成(必須順序):
  1. frontmatter: `title`, `description`, sidebar order
  2. 「今日のゴール」箇条書き
  3. 解説(概念 → 実コードのウォークスルー。コードブロックはリポジトリの実ファイルパスをキャプションで明記)
  4. 「手を動かす」: 実行コマンドと期待出力
  5. クイズ(後述の Quiz コンポーネントで 3〜5 問)
  6. 「今日のまとめ」と「明日の予告」
- クイズは MDX から `<Quiz>` コンポーネントを使う:
  ```mdx
  import Quiz from '../../components/Quiz.astro';

  <Quiz question="..." choices={['A...','B...','C...','D...']} answer={1} explanation="なぜBが正解か..." />
  ```
  (`answer` は 0-indexed。`explanation` は正解表示時に出る解説。)
- 完了条件: `cd docs && npm run build` が通ること。

## k6 規約 (load/)
- スクリプト名は `dayNN-<name>.js`。`BASE_URL` を環境変数(`__ENV.BASE_URL`、デフォルト `http://localhost:8080`)で受ける。
- 短縮作成→リダイレクトの現実的なシナリオ。thresholds(p95 レイテンシ等)を必ず定義。
- 実行方法と「見るべき数値」をスクリプト冒頭コメントに書く。

## 演習ハーネス (study / study.d/)

このリポジトリは「読む教材」ではなく「手を動かす教材」。学習者は `./study start N` で
`work/` に骨組みを受け取り、実装して `./study test` を通す。

日を追加・変更したときは、次も併せて更新すること。

- `study` の `day_title()` と `day_kind()` に、その日を追加する
- `study.d/dayNN.sh` にその日の合格条件を書く（無ければ `default.sh` が使われ、
  Go の build/vet/test だけを見る）。Go 以外が成果物の日は必ず個別に書く
- 合格条件は「学習者が読んで意味が分かる」文言にする。`ok`/`ng` に渡す文字列がそのまま
  学習者へのフィードバックになる
- テストが到達しない実装（DB接続など）は `check_no_todo` が最後の砦になる
- 追加したら、その日で「開始時は落ちる → 模範解答で通る」を必ず確認する

教材ページ (`docs/src/content/docs/dayNN.mdx`) は、実装本体を `<details>` で畳み、
先に学習者が書けるようにすること。冒頭に「## 始め方」「## 今日つくるもの」を置く。

## 禁止事項（追加）
- `day01`〜`day15` のタグを書き換えない。`./study` の土台になっている。

## 禁止事項
- `git commit` / `git push` はしない(コミットはオーケストレーターが行う)。
- `plans/` と自分のタスク範囲外のディレクトリを変更しない。
- プレースホルダ(「ここに解説を書く」等)を残さない。全文を完成させる。
