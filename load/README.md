# load

このディレクトリには k6 の負荷試験スクリプトを置きます。Day 7 ではベースラインを測り、Day 8 以降の改善と比較します。

## k6 のインストール

macOS:

```sh
brew install k6
```

その他の環境では k6 公式ドキュメントの手順に従ってください。

## 実行方法

アプリを起動してから、リポジトリルートで実行します。

```sh
k6 run load/day07-baseline.js
```

接続先を変える場合は `BASE_URL` を指定します。

```sh
BASE_URL=http://localhost:8080 k6 run load/day07-baseline.js
```

## 結果の読み方

`http_req_duration` の `p(95)` は、全リクエストの 95% がその時間以内に終わったことを示します。平均よりも、遅いリクエストの影響を見つけやすい値です。

`iterations` は k6 の default 関数が何回実行されたかです。この教材の Day 7 スクリプトでは、1 iteration ごとに GET か POST を 1 回実行します。

`http_reqs` は HTTP リクエスト数です。setup の作成リクエストも含め、アプリに何回アクセスしたかを確認できます。

## 計測結果メモ

| Day | 条件 | p95 (ms) | iterations | http_reqs | メモ |
| --- | --- | ---: | ---: | ---: | --- |
| Day7 baseline | キャッシュなし、単体アプリ |  |  |  |  |
| Day8 cache | Redis キャッシュあり |  |  |  |  |
| Day11 scale | 複数レプリカ / HPA |  |  |  |  |
