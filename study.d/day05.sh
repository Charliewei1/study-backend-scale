#!/bin/bash
# Day05: Dockerでコンテナ化。
# 成果物は Dockerfile なので、go test ではなく「イメージが要件を満たすか」で判定します。
DF="$WORK/deploy/Dockerfile"
source "$ROOT/study.d/common.sh"
fail=0

echo "→ 骨組みが埋まっているか"
check_no_todo
echo
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }
ng()   { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }

echo "→ 骨組みが埋まっているか"
check_no_todo
echo
echo "→ Dockerfile の中身"
[ -f "$DF" ] || { ng "deploy/Dockerfile がまだありません"; exit 1; }

[ "$(grep -ci '^FROM' "$DF")" -ge 2 ] \
  && ok "multi-stage build になっている（FROMが2つ以上）" \
  || ng "multi-stage build にする（ビルド用と実行用でFROMを分ける）"

grep -qi 'CGO_ENABLED=0' "$DF" \
  && ok "CGO_ENABLED=0 が指定されている" \
  || ng "CGO_ENABLED=0 を指定する（静的リンクにしないとdistrolessで動かない）"

grep -qiE '^\s*USER\s+' "$DF" \
  && ok "非rootユーザーで実行している" \
  || ng "USER を指定する（rootのまま動かさない）"

grep -qi 'distroless\|scratch\|alpine' "$DF" \
  && ok "実行用イメージが小さいベースになっている" \
  || ng "実行用のFROMを distroless / scratch などにする"

if [ "$FULL" != "1" ]; then
  echo
  echo "  （./study test --full で、実際に docker build して起動まで確かめます）"
  exit $fail
fi

command -v docker >/dev/null || { ng "docker が見つかりません"; exit 1; }

echo
echo "→ 実際にビルドする（少し時間がかかります）"
docker build -q -f "$DF" -t shortlink:study "$WORK/app" >/dev/null 2>&1 \
  && ok "docker build が成功した" \
  || { ng "docker build が失敗した"; exit 1; }

size=$(docker image inspect shortlink:study --format '{{.Size}}')
mb=$((size / 1000000))
if [ "$mb" -lt 50 ]; then ok "イメージサイズ ${mb}MB（50MB未満）"
else ng "イメージサイズ ${mb}MB — multi-stageが効いていない可能性があります"; fi

echo
echo "→ 起動して叩いてみる"
cid=$(docker run -d -p 18080:8080 shortlink:study 2>/dev/null)
sleep 2
if curl -sf http://localhost:18080/healthz >/dev/null 2>&1; then
  ok "コンテナが起動して /healthz が応答した"
else
  ng "コンテナに繋がらない（docker logs $cid で確認できます）"
fi
docker rm -f "$cid" >/dev/null 2>&1

exit $fail
