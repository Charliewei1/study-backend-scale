#!/bin/bash
# k8s の日で共通に使う小道具。
ok() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
ng() { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }

# クラスタが無くても確かめられる範囲を見る。
#   - YAMLとして読めるか
#   - apiVersion / kind / metadata.name が揃っているか
# 実際にAPIサーバへ通るかは --full（kind）で確かめます。
check_manifests() {
  local dir="$1"
  [ -d "$dir" ] || { ng "$dir がまだありません"; return; }

  local out
  out=$(python3 - "$dir" <<'PY'
import sys, pathlib, yaml
bad, n = [], 0
for path in sorted(pathlib.Path(sys.argv[1]).glob("*.yaml")):
    try:
        docs = [d for d in yaml.safe_load_all(path.read_text()) if d]
    except yaml.YAMLError as e:
        bad.append(f"{path.name}: YAMLとして読めません ({str(e).splitlines()[0]})")
        continue
    if not docs:
        bad.append(f"{path.name}: 中身が空です")
        continue
    for d in docs:
        n += 1
        if not isinstance(d, dict):
            bad.append(f"{path.name}: マッピングになっていません"); continue
        for key in ("apiVersion", "kind"):
            if not d.get(key):
                bad.append(f"{path.name}: {key} がありません")
        if not (d.get("metadata") or {}).get("name"):
            bad.append(f"{path.name}: metadata.name がありません")
print(n)
for b in bad:
    print("NG " + b)
PY
)
  local count
  count=$(echo "$out" | head -1)
  if echo "$out" | grep -q '^NG '; then
    echo "$out" | grep '^NG ' | sed 's/^NG /  /' | while read -r l; do ng "$l"; done
    fail=1
  elif [ "${count:-0}" -gt 0 ]; then
    ok "${count}個のリソースがマニフェストとして正しく読めた"
  else
    ng "$dir にマニフェストがありません"
  fi
}

want() {
  local dir="$1" pattern="$2" label="$3"
  if grep -rqE "$pattern" "$dir" 2>/dev/null; then ok "$label"; else ng "$label"; fi
}
