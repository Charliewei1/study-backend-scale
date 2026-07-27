#!/bin/bash
# Day06: Docker Compose と Postgres 移行。
# Postgres実装はテストがDBを必要とするので、構成の確認で補います。
source "$ROOT/study.d/common.sh"
fail=0

echo "→ 骨組みが埋まっているか"
check_no_todo

echo
echo "→ Goのコード"
(cd "$WORK/app" && go build ./... && go vet ./...) 2>&1 && ok "build / vet が通った" || ng "build か vet が通りません"
(cd "$WORK/app" && go test ./... >/dev/null 2>&1) && ok "テストが緑" || ng "テストが落ちています"
grep -rq 'pgxpool\|pgx' "$WORK/app/internal/storage" 2>/dev/null \
  && ok "pgx で Postgres に接続している" || ng "internal/storage に Postgres 実装を追加する"
grep -rq 'STORAGE' "$WORK/app" 2>/dev/null \
  && ok "STORAGE 環境変数で保存先を切り替えられる" || ng "STORAGE で memory/sqlite/postgres を切り替える"

echo
echo "→ compose.yaml"
C="$WORK/deploy/compose.yaml"
if [ -f "$C" ]; then
  python3 -c "import yaml,sys; yaml.safe_load(open('$C'))" 2>/dev/null \
    && ok "YAMLとして読める" || ng "YAMLとして読めません"
  grep -q 'postgres' "$C" && ok "postgres サービスがある" || ng "postgres サービスを定義する"
  grep -q 'healthcheck' "$C" && ok "healthcheck がある" || ng "postgres に healthcheck を付ける"
  grep -q 'condition' "$C" && ok "depends_on の condition で起動順を制御している" \
    || ng "depends_on に condition: service_healthy を付ける（起動順の問題を防ぐ）"
  grep -q 'volumes' "$C" && ok "volume でデータを永続化している" || ng "volume を定義する"
else
  ng "deploy/compose.yaml がまだありません"
fi

exit $fail
