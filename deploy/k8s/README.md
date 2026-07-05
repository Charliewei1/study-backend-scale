# Day 9: Kubernetes with kind

この手順は `deploy` ディレクトリから実行します。Day 9 では Kubernetes の基本構造に集中するため、アプリは `STORAGE=memory` で起動します。

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
```

## 5. port-forward でローカルから接続する

```sh
kubectl port-forward svc/shortlink 8080:80 -n shortlink
```

別のターミナルで health check を確認します。

```sh
curl http://localhost:8080/healthz
```

期待する応答は `ok` です。

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

## 後片付け

マニフェストだけ削除する場合:

```sh
kubectl delete -f k8s/base
```

kind クラスタごと削除する場合:

```sh
kind delete cluster --name shortlink
```
