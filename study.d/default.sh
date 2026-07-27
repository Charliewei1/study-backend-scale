#!/bin/bash
# Goが主役の日の合格条件。
source "$ROOT/study.d/common.sh"
fail=0
cd "$WORK/app" || exit 1

race=""
[ "$DAY" \> "03" ] && race="-race"   # Day04以降は並行処理が入るのでrace detectorを付ける

echo "→ 骨組みが埋まっているか"
check_no_todo

echo
echo "→ go build ./..."
go build ./... 2>&1 && ok "ビルドが通った" || ng "ビルドが通りません"

echo
echo "→ go vet ./..."
go vet ./... 2>&1 && ok "vet が黙った" || ng "vet が指摘しています"

echo
echo "→ go test $race ./..."
if go test $race ./... 2>&1 | grep -vE '^(ok|\?)' | head -20; then :; fi
go test $race ./... >/dev/null 2>&1 && ok "テストが緑" || ng "テストが落ちています"

exit $fail
