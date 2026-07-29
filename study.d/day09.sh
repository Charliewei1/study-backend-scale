#!/bin/bash
# Day09: Kubernetes入門。成果物はマニフェスト。
source "$ROOT/study.d/k8s-common.sh"
fail=0
D="$WORK/deploy/k8s/base"

echo "→ マニフェストの構文"
check_manifests "$D"

echo
echo "→ 必要なものが揃っているか"
want "$D" 'kind:\s*Namespace'  "Namespace を定義している"
want "$D" 'kind:\s*Deployment' "Deployment を定義している"
want "$D" 'kind:\s*Service'    "Service を定義している"
want "$D" 'replicas:\s*[2-9]'  "replicas を2以上にしている（1台では冗長化にならない）"
want "$D" 'selector'           "label selector で Service と Pod を結びつけている"

echo
echo "→ kind の設定"
K="$WORK/deploy/kind-config.yaml"
if [ -f "$K" ] && ! grep -q '^# TODO:' "$K"; then
  python3 -c "import yaml,sys; yaml.safe_load(open('$K'))" 2>/dev/null \
    && ok "kind-config.yaml が YAML として読める" || ng "kind-config.yaml が YAML として読めません"
  grep -q 'kind:\s*Cluster' "$K" && ok "kind: Cluster を宣言している" || ng "kind: Cluster を宣言する"
  grep -q 'role:\s*control-plane' "$K" && ok "control-plane ノードを定義している" || ng "control-plane ノードを定義する"
else
  ng "deploy/kind-config.yaml がまだありません"
fi

if [ "$FULL" != "1" ]; then
  echo
  echo "  （./study test --full で、kind クラスタに実際に載せて確かめます）"
  exit $fail
fi

command -v kind >/dev/null || { ng "kind が見つかりません"; exit 1; }
echo
echo "→ kind クラスタで実際に動かす（数分かかります）"
# 学習者が書いた kind-config.yaml を実際に使ってクラスタを作る
if ! kind get clusters 2>/dev/null | grep -q '^shortlink$'; then
  if [ -f "$K" ] && ! grep -q '^# TODO:' "$K"; then
    kind create cluster --name shortlink --config "$K" >/dev/null 2>&1 \
      && ok "自分の kind-config.yaml でクラスタを作成した" \
      || ng "kind-config.yaml でのクラスタ作成に失敗しました"
  else
    kind create cluster --name shortlink >/dev/null 2>&1
  fi
fi
docker build -q -f "$WORK/deploy/Dockerfile" -t shortlink:dev "$WORK/app" >/dev/null 2>&1 \
  && kind load docker-image shortlink:dev --name shortlink >/dev/null 2>&1 \
  && ok "イメージをクラスタに読み込んだ" || ng "イメージの読み込みに失敗"

kubectl apply -f "$D" >/dev/null 2>&1 && ok "apply できた" || ng "apply に失敗"
if kubectl -n shortlink rollout status deploy/shortlink --timeout=90s >/dev/null 2>&1; then
  ok "Pod が Ready になった"
else
  ng "Pod が立ち上がりません（kubectl -n shortlink describe pod で確認）"
fi
exit $fail
