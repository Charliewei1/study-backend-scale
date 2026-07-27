#!/bin/bash
# Day07: 計測。成果物は k6 スクリプトと「数値を読めること」。
S="$WORK/load/day07-baseline.js"
source "$ROOT/study.d/common.sh"
fail=0

echo "→ 骨組みが埋まっているか"
check_no_todo
echo
ok() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
ng() { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }

echo "→ 骨組みが埋まっているか"
check_no_todo
echo
echo "→ k6 スクリプト"
[ -f "$S" ] || { ng "load/day07-baseline.js がまだありません"; exit 1; }

grep -q 'thresholds' "$S" \
  && ok "thresholds が定義されている（合格ラインを決めている）" \
  || ng "thresholds を定義する（p95レイテンシなど、何をもって合格とするか）"

grep -q '__ENV.BASE_URL' "$S" \
  && ok "BASE_URL を環境変数で受けている" \
  || ng "__ENV.BASE_URL で対象を切り替えられるようにする"

grep -qE 'stages|ramping|vus' "$S" \
  && ok "負荷のかけ方（VU/stages）が書かれている" \
  || ng "VU数か stages を定義する"

echo
echo "→ pprof"
if grep -rq 'net/http/pprof' "$WORK/app" 2>/dev/null; then
  ok "net/http/pprof が組み込まれている"
else
  ng "net/http/pprof を DEBUG_ADDR で有効にできるようにする"
fi

if [ "$FULL" != "1" ]; then
  echo
  echo "  （./study test --full で、実際にサーバを起動して k6 を走らせます）"
  exit $fail
fi

command -v k6 >/dev/null || { ng "k6 が見つかりません"; exit 1; }

echo
echo "→ サーバを起動して実際に負荷をかける"
(cd "$WORK/app" && PORT=18081 go run ./cmd/server >/tmp/study-k6-server.log 2>&1) &
srv=$!
sleep 4
if BASE_URL=http://localhost:18081 k6 run --quiet "$S" 2>&1 | tail -25; then
  ok "k6 が thresholds を満たして完走した"
else
  ng "thresholds を満たしませんでした（数値を読んでボトルネックを考えましょう）"
fi
kill $srv 2>/dev/null; wait $srv 2>/dev/null

exit $fail
