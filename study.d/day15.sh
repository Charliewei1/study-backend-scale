#!/bin/bash
# Day15: 総仕上げ。全部載せの構成が揃っているか。
source "$ROOT/study.d/k8s-common.sh"
fail=0
D="$WORK/deploy/k8s/full"

echo "→ フル構成のマニフェスト"
check_manifests "$D"
want "$D" 'postgres' "Postgres が含まれている"
want "$D" 'redis'    "Redis が含まれている"
want "$D" 'kind:\s*HorizontalPodAutoscaler' "HPA が含まれている"
want "$D" 'kind:\s*PersistentVolumeClaim'   "PVC で永続化している"

echo
echo "→ 最終計測スクリプト"
if [ -f "$WORK/load/day15-final.js" ]; then
  ok "load/day15-final.js がある"
  grep -q 'thresholds' "$WORK/load/day15-final.js" \
    && ok "thresholds が定義されている（Day07と比較できる）" \
    || ng "Day07と同条件で比べられるよう thresholds を定義する"
else
  ng "load/day15-final.js がありません"
fi

echo
echo "→ Goのコード"
(cd "$WORK/app" && go build ./... && go vet ./... && go test ./... >/dev/null 2>&1) \
  && ok "go build / vet / test が通った" || ng "Goのテストが通っていません"

exit $fail
