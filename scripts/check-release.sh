#!/usr/bin/env bash
# check-release.sh —— 发版前置校验（CI 与本地共用同一份判据）。
#
# 为什么要有：Build Release 是 tag 触发的，一个手滑的 tag（版本常量没 bump、
# RELEASES.md 没写段落、决议项范围标记没同步）会直接产出一个「说明空白 / 版本号对不上」
# 的正式 release，且因为 tag 一旦被消费就不好重发。把这三条判据放到构建之前，
# 失败即停，比发出去再删 tag 便宜得多。
#
# 用法：TAG=v2.9.7 bash scripts/check-release.sh
#   判据全部只读（不写任何文件），可在本地发版前手动跑一遍。
set -euo pipefail

if [ -z "${TAG:-}" ]; then
  echo "TAG 未设置，用法：TAG=vX.Y.Z bash scripts/check-release.sh" >&2
  exit 2
fi
VER="${TAG#v}"

fail() { echo "::error::$*"; echo "✗ $*" >&2; exit 1; }
ok()   { echo "✓ $*"; }

[ -f RELEASES.md ] || fail "仓库根目录缺少 RELEASES.md"
[ -f internal/app/app.go ] || fail "缺少 internal/app/app.go（版本常量所在地）"

# 1) 版本常量必须等于 tag（去 v 前缀）
APP_VER="$(grep -oE 'const Version = "[^"]+"' internal/app/app.go | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || true)"
[ -n "${APP_VER}" ] || fail "internal/app/app.go 里找不到 const Version = \"...\""
[ "${APP_VER}" = "${VER}" ] || fail "版本常量 ${APP_VER} 与 tag ${TAG} 不一致（发版前先 bump internal/app/app.go）"
ok "版本常量 ${APP_VER} 与 tag ${TAG} 一致"

# 2) RELEASES.md 必须已有该版本段落（release note 从这里截取）
if grep -qE "^# wild-work ${TAG}([^0-9.]|$)" RELEASES.md; then
  ok "RELEASES.md 含 ${TAG} 段落"
else
  fail "RELEASES.md 缺少 \"# wild-work ${TAG}\" 段落（发版说明必须先写到文件末尾）"
fi

# 3) 决议项范围标记三处必须一致：本仓纪律要求改号「四处同步」，
#    范围标记是其中可被机器校验的两处半（索引行与文档标题 + 文档表），漏改会让文档自相矛盾。
N1="$(grep -oE '决议项索引（R1–R[0-9]+）' AGENTS.md | head -1 | grep -oE 'R1–R[0-9]+' || true)"
N2="$(head -1 docs/决议项.md | grep -oE 'R1–R[0-9]+' | tail -1 || true)"
N3="$(grep -oE '\*\*R1–R[0-9]+ 决议全文\*\*' AGENTS.md | head -1 | grep -oE 'R1–R[0-9]+' || true)"
[ -n "${N1}" ] && [ -n "${N2}" ] && [ -n "${N3}" ] || fail "决议项范围标记缺失（AGENTS.md §1 / AGENTS.md §6 / docs/决议项.md:1）"
[ "${N1}" = "${N2}" ] && [ "${N1}" = "${N3}" ] || fail "决议项范围标记不一致：AGENTS.md §1=${N1}, docs 标题=${N2}, AGENTS.md §6=${N3}"
ok "决议项范围标记一致（${N1}）"

echo "发版前置校验通过：${TAG}"
