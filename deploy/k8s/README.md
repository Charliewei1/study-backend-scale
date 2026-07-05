# Day 9-10: Kubernetes with kind

この手順は `deploy` ディレクトリから実行します。base は Kubernetes の基本構造に集中するため、アプリは `STORAGE=memory` で起動します。Day 10 では ConfigMap/Secret、liveness/readiness probe、resources、ローリング更新、教材用 PostgreSQL manifest を追加します。

```sh
cd deploy
```

## 1. kind クラスタを作成する

```sh
kind create cluster --name shortlink --config kind-config.yaml
```

`kind-config.yaml` は教材用のシンプルな 1 ノード構成です。

## 2. Docker イメージをビルドする

```sh
docker build -f Dockerfile -t shortlink:dev ../app
```

## 3. イメージを kind クラスタに読み込む

```sh
kind load docker-image shortlink:dev --name shortlink
```

`Deployment` は `imagePullPolicy: Never` を指定しているため、外部レジストリではなく kind に読み込んだローカルイメージを使います。

## 4. Kubernetes マニフェストを適用する

```sh
kubectl apply -f k8s/base
```

作成されたリソースを確認します。

```sh
kubectl get all -n shortlink
kubectl get configmap,secret -n shortlink
```

## 5. port-forward でローカルから接続する

```sh
kubectl port-forward svc/shortlink 8080:80 -n shortlink
```

別のターミナルで health check を確認します。

```sh
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

期待する応答は `ok` です。

`/healthz` は liveness 用で、プロセスが HTTP 応答できるかだけを見ます。`/readyz` は readiness 用で、storage に ping して Service の転送先に入れてよいかを見ます。

リンク作成も確認できます。

```sh
curl -i -X POST http://localhost:8080/api/links \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
```

## kubectl の練習コマンド

基本の一覧表示です。

```sh
kubectl get namespace
kubectl get deploy,rs,pod,svc -n shortlink
kubectl get pod -n shortlink -l app.kubernetes.io/name=shortlink
```

`describe` はイベントや selector、Service の転送先確認に便利です。

```sh
kubectl describe deployment shortlink -n shortlink
kubectl describe service shortlink -n shortlink
kubectl describe pod -n shortlink -l app.kubernetes.io/name=shortlink
```

`logs` は Pod 内のアプリログを確認します。Pod が 2 つあるため、ラベル指定でまとめて見られます。

```sh
kubectl logs -n shortlink -l app.kubernetes.io/name=shortlink
kubectl logs -n shortlink -l app.kubernetes.io/name=shortlink --tail=50
```

`exec` は Pod 内でコマンドを実行します。distroless イメージには shell が無いため、次のコマンドは失敗します。この失敗から「本番用イメージには余計なツールを入れない」ことを観察します。

```sh
POD_NAME=$(kubectl get pod -n shortlink -l app.kubernetes.io/name=shortlink -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n shortlink "$POD_NAME" -- /bin/sh
```

`exec: "/bin/sh": stat /bin/sh: no such file or directory` のようなエラーが出れば、distroless には `sh` が無いことを確認できています。

## PostgreSQL へ切り替える

base は `STORAGE=memory` のままです。Kubernetes 内の教材用 PostgreSQL を使う場合は、base を適用した後に `k8s/postgres` を適用します。

```sh
kubectl apply -f k8s/postgres
```

`k8s/postgres/01-app-configmap.yaml` は `shortlink-config` を上書きし、`STORAGE=postgres` に切り替えます。base の `shortlink-secret` には教材用ダミーの `DATABASE_URL` が入っています。本物の接続文字列やパスワードはリポジトリに入れないでください。

ConfigMap の変更は起動済み Pod の環境変数には自動反映されないため、アプリ Pod を再作成します。

```sh
kubectl rollout restart deployment/shortlink -n shortlink
kubectl rollout status deployment/shortlink -n shortlink
```

PostgreSQL とアプリの状態を確認します。

```sh
kubectl get deploy,pod,svc,pvc -n shortlink
kubectl describe pod -n shortlink -l app.kubernetes.io/name=shortlink
kubectl logs -n shortlink -l app.kubernetes.io/name=postgres --tail=50
```

memory に戻す場合は base の ConfigMap を再適用し、同じく rollout restart します。

```sh
kubectl apply -f k8s/base/01-configmap.yaml
kubectl rollout restart deployment/shortlink -n shortlink
kubectl rollout status deployment/shortlink -n shortlink
```

## ローリング更新を観察する

Deployment は `RollingUpdate`、`maxSurge: 1`、`maxUnavailable: 0` です。新しい Pod が `/readyz` を通るまで古い Pod を残すため、ready な Pod 数を減らさずに更新できます。

まず状態を別ターミナルで見続けます。

```sh
kubectl get pod -n shortlink -w
```

別のターミナルでイメージタグを更新して rollout を発生させます。実際に試す場合は、先に新しいタグのイメージを build して kind に load してください。

```sh
docker build -f Dockerfile -t shortlink:day10 ../app
kind load docker-image shortlink:day10 --name shortlink
kubectl set image deployment/shortlink shortlink=shortlink:day10 -n shortlink
kubectl rollout status deployment/shortlink -n shortlink
```

更新履歴を確認します。

```sh
kubectl rollout history deployment/shortlink -n shortlink
```

問題があった更新は undo で直前の ReplicaSet に戻せます。

```sh
kubectl rollout undo deployment/shortlink -n shortlink
kubectl rollout status deployment/shortlink -n shortlink
```

## HPA を観察する

Day 11 では `k8s/base/03-hpa.yaml` で HorizontalPodAutoscaler を追加します。HPA は metrics-server が集める CPU 使用率を使うため、kind では先に metrics-server を入れます。次の URL は、この教材で検証済みの v0.8.1 に固定しています。

```sh
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/download/v0.8.1/components.yaml
```

kind の kubelet 証明書はローカル教材用のため、そのままだと metrics-server が kubelet へ TLS 接続できないことがあります。次の patch で `--kubelet-insecure-tls` を付けます。

```sh
kubectl patch deployment metrics-server -n kube-system --type=json \
  -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
kubectl rollout status deployment/metrics-server -n kube-system
```

metrics が見えるようになったか確認します。

```sh
kubectl top node
kubectl top pod -n shortlink
```

HPA の状態は別ターミナルで見続けます。

```sh
kubectl get hpa -n shortlink -w
```

負荷スクリプトは作成したリンクを後続の GET で読むため、事前に `k8s/postgres` を適用して `STORAGE=postgres` に切り替えておきます。

さらに別ターミナルで port-forward を実行します。

```sh
kubectl port-forward svc/shortlink 8080:80 -n shortlink
```

もう 1 つのターミナルで k6 を実行すると、CPU 使用率が上がったタイミングで `TARGETS` と `REPLICAS` が変わる様子を観察できます。

```sh
BASE_URL=http://localhost:8080 k6 run ../load/day11-scale.js
```

## 後片付け

マニフェストだけ削除する場合:

```sh
kubectl delete -f k8s/postgres --ignore-not-found
kubectl delete -f k8s/base
```

kind クラスタごと削除する場合:

```sh
kind delete cluster --name shortlink
```
