# CURRICULUM.md — 15日間カリキュラム(オーケストレーター作成・正)

テーマ: URL 短縮サービス「shortlink」を 1 日ずつ育て、Go → Docker → Kubernetes → スケーラビリティへ段階的に学ぶ。
各日は「コード増分」+「教材ページ(docs/src/content/docs/dayNN.mdx)」+「必要なら deploy/ / load/ の資産」で構成。
ワーカーは自分の日のセクションと HARNESS.md だけ読めばよい。

---
## Day 1 — Go 入門と最小の URL 短縮サーバ
- コード: `app/` を新規作成。`cmd/server/main.go` + `internal/shortener/shortener.go`(連番 ID → base62 エンコード)+ `internal/storage/memory.go`(`map[string]string` + `sync.RWMutex`)+ `internal/handler/handler.go`。API 契約は HARNESS 参照。`internal/shortener` と `internal/storage` にユニットテスト。
- 教材: Go の思想(シンプルさ・明示性)、モジュール/パッケージ、`net/http` サーバの仕組み(1 リクエスト 1 goroutine)、base62 の理屈(なぜ 62 進数か、6 文字で 62^6≈568 億)。curl での動作確認。
- クイズ例: goroutine とスレッドの違い / base62 の容量計算 / ServeMux のパターンマッチ。

## Day 2 — API 設計とテスト文化
- コード: ハンドラを JSON エラーレスポンス(`{"error":"..."}`)に統一、URL バリデーション(`net/url.Parse` + スキーム検査)、`GET /api/links/{code}` でメタ情報取得を追加。`internal/handler` に `httptest` を使った table-driven テストを整備。`Makefile`(`run/test/vet/fmt`)。
- 教材: REST 設計(ステータスコードの使い分け、201 と Location)、Go のテスト哲学(table-driven、`t.Run`)、`httptest` の仕組み。カバレッジの見方。
- クイズ例: 201 vs 200 / table-driven テストの利点 / `t.Parallel` の意味。

## Day 3 — 永続化とインターフェース設計
- コード: `internal/storage/storage.go` に `Storage` インターフェース(`Save/Load/…` + `context.Context`)を定義し、memory 実装をそれに適合。`internal/storage/sqlite.go` を `modernc.org/sqlite`(cgo 不要)で追加。環境変数 `STORAGE=(memory|sqlite)`, `SQLITE_PATH` で切替。sqlite 実装のテスト(t.TempDir)。
- 教材: なぜインターフェースか(依存性の注入、テスト容易性、後日の Postgres/Redis 差し替えの布石)、Go のインターフェースは暗黙的実装、`database/sql` の仕組み(コネクションプール)、`context.Context` の役割。
- クイズ例: 暗黙的インターフェース実装 / `sql.DB` はプールである / context の伝播。

## Day 4 — Go の並行処理とクリック統計
- コード: リダイレクト時のクリック数を記録する `internal/analytics/`: チャネルにイベントを送り、単一 goroutine が集計(バッファ付き ch、ドロップ戦略あり)して Storage に反映。`GET /api/links/{code}/stats` → `{"code":..,"url":..,"clicks":N}`。`-race` 付きテスト、mutex 版との比較をコードコメントで。graceful shutdown(`signal.NotifyContext` + `http.Server.Shutdown`)もここで導入。
- 教材: goroutine/channel/select、「メモリ共有ではなく通信」、race detector デモ(壊れる例→直す)、graceful shutdown がスケール運用で重要な理由。
- クイズ例: バッファ付きチャネルの挙動 / race condition の定義 / Shutdown が待つもの。

## Day 5 — Docker でコンテナ化
- コード/資産: `deploy/Dockerfile`(multi-stage: golang:1.24 build → `gcr.io/distroless/static-debian12` run、CGO_ENABLED=0、非 root)、`app/.dockerignore`。README に build/run 手順。イメージサイズを段階別に比較するデモ(single-stage 版を教材内で対比)。
- 教材: コンテナとは(namespace/cgroups をざっくり)、イメージレイヤとキャッシュ、multi-stage build でサイズが ~1GB→~10MB になる理屈、distroless の利点、タグ運用。
- クイズ例: レイヤキャッシュが効く条件 / CGO_ENABLED=0 の意味 / distroless に shell が無い意味。

## Day 6 — Docker Compose と Postgres 移行
- コード/資産: `internal/storage/postgres.go`(`github.com/jackc/pgx/v5/pgxpool`)。`STORAGE=postgres`, `DATABASE_URL`。起動時マイグレーション(`schema.sql` 埋め込み or CREATE TABLE IF NOT EXISTS)。`deploy/compose.yaml`: app + postgres(healthcheck, depends_on condition, volume)。
- 教材: Compose の宣言的環境、サービスディスカバリ(サービス名 DNS)、healthcheck と起動順序、なぜ本番スケールでは SQLite でなく Postgres か(同時書き込み、ネットワーク越し共有)。
- クイズ例: depends_on + condition / volume の永続化 / pgxpool の利点。

## Day 7 — 計測: k6 ベースラインと pprof
- 資産: `load/day07-baseline.js`(create 10% / redirect 90% の混合シナリオ、ramping-vus、thresholds p95<100ms)。`app` に `net/http/pprof` を `DEBUG_ADDR`(別ポート、デフォルト無効)で追加。`docs` に計測結果の読み方と記録テンプレート。
- 教材: 「推測するな、計測せよ」、スループット/レイテンシ/パーセンタイルの定義(平均が嘘をつく話)、k6 の VU モデル、pprof フレームグラフの読み方。ここで測ったベースラインを以降の日で比較していく宣言。
- クイズ例: p95 の意味 / VU とは / ボトルネック特定の順序。

## Day 8 — Redis キャッシュで読みを速くする
- コード/資産: `internal/cache/redis.go`(`github.com/redis/go-redis/v9`)。cache-aside: redirect 時 code→URL を Redis から引き、ミスなら DB→SET(TTL 24h)。`REDIS_ADDR` 未設定ならキャッシュ無効(素通し)。compose に redis 追加。`load/day08-cached.js` で before/after 比較。
- 教材: read-heavy ワークロード(短縮 URL は読み:書き ≈ 100:1)、cache-aside パターン、TTL と無効化(「キャッシュ無効化は計算機科学の二大難問」)、キャッシュヒット率の観測。k6 での改善数値例を提示。
- クイズ例: cache-aside の読みフロー / TTL 設計 / キャッシュが効かないケース。

## Day 9 — Kubernetes 入門: kind で動かす
- 資産: `deploy/k8s/base/`: `namespace.yaml`, `deployment.yaml`(replicas:2, image shortlink:dev), `service.yaml`(ClusterIP)。`deploy/kind-config.yaml`(port mapping)。手順: kind クラスタ作成 → `kind load docker-image` → apply → port-forward で確認。Postgres は Day9 時点では compose のままでもよく、k8s 内は `STORAGE=memory` で起動して構造の学習に集中。
- 教材: k8s とは何を解決するのか(宣言的・自己修復・スケジューリング)、Pod/Deployment/ReplicaSet/Service の関係図、label selector、kubectl 基本動詞(get/describe/logs/exec)。
- クイズ例: Deployment と ReplicaSet の関係 / Service が Pod を見つける仕組み / 宣言的の意味。

## Day 10 — k8s 設定・ヘルスチェック・ローリング更新
- 資産: k8s マニフェスト拡張: ConfigMap(PORT等), Secret(DATABASE_URL), liveness/readiness probe(/healthz), resources requests/limits, RollingUpdate 戦略(maxSurge/maxUnavailable)。`app` に `GET /readyz`(storage ping 確認)を追加。Postgres を k8s 内に StatefulSet…はやり過ぎなので、Bitnami等は使わず単純な postgres Deployment+PVC を提供。
- 教材: liveness と readiness の違い(混同すると再起動ループ)、resources と QoS クラス、ローリング更新が無停止になる仕組み(readiness gate との連携)、ConfigMap/Secret の注入方法。
- クイズ例: liveness 誤設定の症状 / requests と limits / maxUnavailable=0 の意味。

## Day 11 — 水平スケールと HPA
- コード/資産: ID 採番を「連番」から「ランダム 7 文字 base62 + 衝突リトライ」へ変更(複数レプリカで連番カウンタが破綻するため — ステートレス化の核心)。`deploy/k8s/base/hpa.yaml`(cpu 70%)。metrics-server 導入手順(kind 用 patch)。`load/day11-scale.js` で HPA 発動を観察する手順。
- 教材: 垂直 vs 水平スケール、ステートレス設計(なぜローカルメモリのカウンタ/セッションがダメか)、分散 ID 採番の選択肢(ランダム+リトライ / 採番サービス / Snowflake)と誕生日問題の衝突確率、HPA のアルゴリズム。
- クイズ例: ステートレスの定義 / 衝突確率の直感 / HPA の desiredReplicas 式。

## Day 12 — 可観測性: Prometheus と structured logging
- コード/資産: `promhttp` で `/metrics`(リクエスト数・レイテンシヒストグラム・キャッシュヒット率のカスタムメトリクス、`internal/metrics/`)。`log/slog` で JSON 構造化ログ+リクエストロガー middleware。`deploy/compose.monitoring.yaml`(prometheus + grafana、スクレイプ設定、簡単なダッシュボード JSON)。
- 教材: メトリクス/ログ/トレースの三本柱、RED メソッド、ヒストグラムと分位数(なぜ平均でなく histogram_quantile か)、構造化ログが grep より強い理由、PromQL 入門(rate, histogram_quantile)。
- クイズ例: counter と gauge / rate() の意味 / RED の R・E・D。

## Day 13 — レートリミットと回復性
- コード: `internal/middleware/ratelimit.go`(token bucket、`golang.org/x/time/rate`、クライアント IP 別、`X-RateLimit-*` ヘッダ、429)。タイムアウト設計(`http.Server` の Read/Write timeout、ハンドラの context timeout、DB/Redis 呼び出しのタイムアウト)。リトライ+指数バックオフの解説(コードは Redis 接続で例示)。`load/day13-ratelimit.js` で 429 を観察。
- 教材: なぜ守りが要るか(自衛としてのレートリミット、カスケード障害)、token bucket vs sliding window、タイムアウトの連鎖設計、サーキットブレーカー概念、graceful degradation(Redis 死→DB 直行は Day8 で実装済み、を回収)。
- クイズ例: token bucket の burst / 429 と Retry-After / タイムアウトを内側ほど短くする理由。

## Day 14 — さらなるスケール: シャーディングと分散の理論
- コード/資産: `internal/cluster/hashring.go` に consistent hashing のミニ実装(仮想ノード付き)+テスト(ノード追加時の再配置率を検証するテスト)。教材中心の日。
- 教材: 1 台の DB の限界→リードレプリカ(レプリケーションラグと read-your-writes)、シャーディング(range vs hash、リシャーディング問題)、consistent hashing(なぜ mod N がダメか、仮想ノード)、CAP 定理をやさしく、CDN/エッジキャッシュ(URL 短縮は 301 キャッシュと相性が良い話)。実装した hashring のウォークスルー。
- クイズ例: mod N の欠点 / レプリケーションラグの症状 / CAP の P は選べない。

## Day 15 — 総仕上げ: フルスタックデプロイと最終計測
- 資産: `deploy/k8s/full/` に全部載せ(app + postgres + redis + HPA + monitoring は compose 併用でも可)を kustomize なしの素朴な YAML 一式で。`load/day15-final.js`(day07 と同条件)+ 総合レポートページ: Day7 ベースライン → Day8 キャッシュ → Day11 スケールの数値変化まとめ(教材内に例示値の表)。全体アーキテクチャ図(mermaid)。卒業クイズ 10 問。今後の学習ロードマップ(gRPC, service mesh, Kafka, マネージド k8s…)。
- 教材: 15 日の総復習ストーリー(1 バイナリ → コンテナ → オーケストレーション → 計測駆動改善)、「スケーラビリティとは負荷に比例してリソースを足せば性能が伸びる性質」の再定義。
