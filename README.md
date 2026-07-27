# study-backend-scale — URL 短縮で学ぶ Go / Docker / Kubernetes / スケーラビリティ

URL 短縮サービス **shortlink** を 15 日間かけて育てながら、バックエンドのスケーラビリティを実践的に学ぶ教材リポジトリです。

読むだけの教材ではありません。毎日、骨組みを受け取って自分で実装し、合格条件を満たして次の日へ進みます。

## 始め方

```bash
./study start 1
```

`work/` にその日の演習が用意されます。`panic("TODO: ...")` が残っている場所が今日の課題です。

```bash
./study test      # まず落ちることを確認 → work/ を埋める → 通るまで直す
```

```bash
./study answer    # 模範解答と見比べる
```

全体の流れは `./study days`、その日の解説は `./study hint` で読めます。

## コマンド

| コマンド | すること |
|---|---|
| `./study` | いまどこにいるか |
| `./study days` | 15日間の一覧 |
| `./study start 3` | Day03の演習を用意する |
| `./study test` | その日の合格条件をチェックする |
| `./study test --full` | 重い検証まで（`docker build` / kind / k6） |
| `./study todo` | 残っているTODO |
| `./study hint` | その日の解説 |
| `./study run` | 書いたサーバを起動する |
| `./study docs` | 解説サイトをローカルで開く |
| `./study answer` | 模範解答との差分 |
| `./study peek <パス>` | 模範解答をそのまま見る |
| `./study reset` | 今日をやり直す |

`./study start` と `./study reset` は、いまの `work/` を `work.prev/` へ退避してから作り直します。
書いたコードがいきなり消えることはありません。

## 合格条件は日によって変わります

後半は成果物がGoのコードとは限りません。`./study test` はその日の主役に合わせて確認内容を変えます。

| Day | 成果物 | 確かめること |
|---|---|---|
| 1〜4, 6, 8, 11〜14 | Goのコード | `go build` / `go vet` / `go test`（Day4以降は `-race`） |
| 5 | Dockerfile | multi-stage、非root、`CGO_ENABLED=0`。`--full` で実ビルドと起動 |
| 7 | k6スクリプト | thresholds と pprof。`--full` で実際に負荷をかける |
| 9, 10, 15 | k8sマニフェスト | 必須リソースと設定。`--full` で kind クラスタに載せる |

## 必要なもの

Go 1.24+ は必須です。以下は該当する日だけ必要になります。

| 道具 | 使う日 |
|---|---|
| Docker | Day 5, 6, 9〜15 |
| kind / kubectl | Day 9, 10, 11, 15 |
| k6 | Day 7, 8, 11, 13, 15 |
| Node.js | 解説サイトをローカルで見る場合のみ |

## 構成

| パス | 内容 |
|---|---|
| `study` | 演習の用意・実行・答え合わせ |
| `study.d/` | 日ごとの合格条件 |
| `app/` | Go 製 URL 短縮サービス（模範解答・日ごとに進化） |
| `deploy/` | Dockerfile / Docker Compose / Kubernetes マニフェスト |
| `load/` | k6 負荷試験スクリプト |
| `docs/` | 教材サイト（Astro Starlight、クイズ付き） |
| `plans/` | カリキュラムとワーカー規約 |
| `tools/skeleton/` | 模範解答から骨組みを生成する裏方 |
| `work/` | あなたの演習場（git管理外） |

各日の完成状態は git タグ `day01`〜`day15` にあります。`./study` はこれを土台にしているので、タグは書き換えないでください。

## 15 日間の旅

Go 入門 → API 設計 → 永続化 → 並行処理 → Docker → Compose+Postgres → k6 計測 → Redis キャッシュ → k8s 入門 → 設定と可用性 → 水平スケール+HPA → 可観測性 → レートリミット → 分散の理論 → 総仕上げ

詳細は [plans/CURRICULUM.md](plans/CURRICULUM.md) と教材サイトを参照してください。

## 教材サイトをローカルで見る

```sh
./study docs
```
