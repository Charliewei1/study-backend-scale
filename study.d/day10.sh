#!/bin/bash
# Day10: k8sの設定・ヘルスチェック・ローリング更新。
# Goの /readyz 実装とマニフェスト、両方が成果物。
source "$ROOT/study.d/common.sh"
source "$ROOT/study.d/k8s-common.sh"
fail=0

echo "→ 骨組みが埋まっているか"
check_no_todo
echo
D="$WORK/deploy/k8s/base"

echo "→ Goのコード"
(cd "$WORK/app" && go build ./... 2>&1 && go vet ./... 2>&1 && go test ./... 2>&1 | grep -E '^(FAIL|ok)') \
  && ok "go build / vet / test が通った" || ng "Goのテストが通っていません"
grep -rq 'readyz' "$WORK/app" && ok "/readyz を実装している" || ng "/readyz を追加する（storageに実際に触れる確認）"

echo
echo "→ マニフェスト"
check_manifests "$D"
want "$D" 'livenessProbe'   "livenessProbe を設定している"
want "$D" 'readinessProbe'  "readinessProbe を設定している"
want "$D" 'kind:\s*ConfigMap' "ConfigMap で設定を外に出している"
want "$D" 'kind:\s*Secret'    "Secret で機密を分けている"
want "$D" 'resources'         "resources requests/limits を指定している"
want "$D" 'maxUnavailable|maxSurge' "RollingUpdate の戦略を明示している"

exit $fail
