# Day 15: full Kubernetes manifests

`deploy/k8s/full/` は、Day 15 の最終構成を kind 上で動作実証するために app、PostgreSQL、Redis、HPA をまとめて動かす素朴な YAML 一式です。kustomize は使いません。`deploy/k8s/base/` とは独立しており、`shortlink-full` Namespace に作るため既存の `shortlink` と衝突しません。

## 1. kind クラスタを用意する

既存の Day 9-11 用クラスタを使っても、新しく作っても構いません。新しく作る場合はリポジトリルートから実行します。

```sh
kind create cluster --name shortlink --config deploy/kind-config.yaml
```

## 2. Docker イメージをビルドする

```sh
docker build -f deploy/Dockerfile -t shortlink:dev app
```

## 3. イメージを kind クラスタに読み込む

```sh
kind load docker-image shortlink:dev --name shortlink
```

`08-app-deployment.yaml` は `imagePullPolicy: Never` を指定しているため、外部レジストリではなく kind に読み込んだローカルイメージを使います。

## 4. マニフェストを適用する

ファイル名は適用順を意識して連番にしています。Namespace、設定、Secret、PostgreSQL、Redis、app、HPA の順です。

```sh
kubectl apply -f deploy/k8s/full
kubectl get all,pvc,configmap,secret -n shortlink-full
```

Pod の準備完了を待ちます。

```sh
kubectl rollout status deployment/postgres -n shortlink-full
kubectl rollout status deployment/redis -n shortlink-full
kubectl rollout status deployment/shortlink -n shortlink-full
```

HPA の CPU 使用率を観察する場合は metrics-server が必要です。kind では次のように入れます。

```sh
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.8.1/components.yaml
kubectl patch deployment metrics-server -n kube-system --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
kubectl rollout status deployment/metrics-server -n kube-system
```

## 5. port-forward でローカルから接続する

```sh
kubectl port-forward svc/shortlink 8080:80 -n shortlink-full
```

別のターミナルで health check、作成 API、リダイレクトを確認します。

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl -i -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

`/healthz` と `/readyz` の期待する応答は `ok` です。`/readyz` は PostgreSQL へ ping するため、DB が準備できるまでは Service の転送先に入りません。
作成レスポンスの JSON に含まれる `code` を使って、`302` と `Location` を確認します。

```sh
curl -i http://localhost:8080/<code>
```

期待する応答は `HTTP/1.1 302 Found` と `Location: https://example.com` です。

## 6. k6 で負荷を流す

Day 15 の負荷スクリプトは Day 7 と同じ条件です。この port-forward 経由の実行は、full 構成へ負荷を流したときの動作確認として扱います。
教材本文の最終比較値は、Day 7 ベースライン、Day 8 キャッシュ、Day 15 最終構成相当を同じ Docker Compose 環境、同じ負荷条件で測った値です。
port-forward 経由の計測はトンネルがボトルネックになりやすいため、公平な性能比較には使いません。

```sh
BASE_URL=http://localhost:8080 k6 run load/day15-final.js
```

HPA の状態を見る場合は別ターミナルで実行します。

```sh
kubectl get hpa -n shortlink-full -w
```

## レートリミットを有効にする場合

full 構成では `RATE_LIMIT_RPS` と `RATE_LIMIT_BURST` を設定していないため、レートリミットは無効です。これはベンチマークを 429 で歪めないためです。
有効化したい場合は、`01-configmap.yaml` の `data:` に次のような値を追記し、`08-app-deployment.yaml` の `env` に同じキーの `configMapKeyRef` を追加します。

```yaml
RATE_LIMIT_RPS: "100"
RATE_LIMIT_BURST: "200"
```

## 後片付け

マニフェストだけ削除する場合:

```sh
kubectl delete -f deploy/k8s/full --ignore-not-found
```

Namespace ごと削除する場合:

```sh
kubectl delete namespace shortlink-full
```

kind クラスタごと削除する場合:

```sh
kind delete cluster --name shortlink
```
