# study-backend-scale — URL 短縮で学ぶ Go / Docker / Kubernetes / スケーラビリティ

URL 短縮サービス **shortlink** を 15 日間かけて育てながら、バックエンドのスケーラビリティを実践的に学ぶ教材リポジトリです。
1 日 = 1 コミット。各日の解説・クイズ付き教材は GitHub Pages(Astro Starlight)で公開されます。

## 構成

| パス | 内容 |
|---|---|
| `app/` | Go 製 URL 短縮サービス(日ごとに進化) |
| `deploy/` | Dockerfile / Docker Compose / Kubernetes マニフェスト |
| `load/` | k6 負荷試験スクリプト |
| `docs/` | 教材サイト(Astro Starlight、クイズ付き) |
| `plans/` | カリキュラムとワーカー規約 |

## 15 日間の旅

Go 入門 → API 設計 → 永続化 → 並行処理 → Docker → Compose+Postgres → k6 計測 → Redis キャッシュ → k8s 入門 → 設定と可用性 → 水平スケール+HPA → 可観測性 → レートリミット → 分散の理論 → 総仕上げ

詳細は [plans/CURRICULUM.md](plans/CURRICULUM.md) と教材サイトを参照してください。

## 教材サイトをローカルで見る

```sh
cd docs && npm install && npm run dev
```

## アプリを動かす

```sh
cd app && go run ./cmd/server
curl -X POST localhost:8080/api/links -d '{"url":"https://example.com"}'
```
