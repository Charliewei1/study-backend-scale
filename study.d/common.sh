#!/bin/bash
ok() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
ng() { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }

# 骨組みの TODO が残っていないか。
# テストが到達しない場所（DB接続など）は、これが最後の砦になります。
check_no_todo() {
  # 走査するのは実装ファイルだけ。生成した HINTS.md / AGENTS.md には
  # 解説文として "TODO" の字が出てくるので、そこは見ない。
  local left
  left=$(grep -rl 'panic("TODO:\|^# TODO:' \
           "$WORK/app" "$WORK/deploy" "$WORK/load" 2>/dev/null |
         sed "s|$WORK/||" | sort)
  if [ -n "$left" ]; then
    ng "まだ骨組みのままのファイルがあります:"
    echo "$left" | sed 's/^/      /'
  else
    ok "TODOはすべて埋まっている"
  fi
}
