#!/bin/sh
#
# tdd-agent installer.
#
# Cài lệnh `tdd-agent` bằng `uv tool install` từ git. Nếu máy chưa có `uv`, script cài `uv` trước
# (uv tự lo Python >= 3.11 nên không cần cài Python riêng).
#
# Repo public:
#   curl -fsSL https://raw.githubusercontent.com/blcvn/tdd-agentic-ai/main/install.sh | sh
#
# Repo private (cần token có quyền đọc repo, ví dụ fine-grained PAT với "Contents: read"):
#   export GITHUB_TOKEN=ghp_xxx
#   curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
#     https://raw.githubusercontent.com/blcvn/tdd-agentic-ai/main/install.sh | sh
#
# Cập nhật:  chạy lại đúng lệnh trên (lấy bản mới nhất của TDD_AGENT_REF).
# Gỡ:        curl -fsSL .../install.sh | sh -s -- --uninstall
#
# Biến môi trường:
#   TDD_AGENT_REF      nhánh, tag hoặc commit cần cài   (mặc định: main)
#   TDD_AGENT_REPO     owner/repo trên GitHub           (mặc định: blcvn/tdd-agentic-ai)
#   TDD_AGENT_SOURCE   nguồn cài tuỳ ý (đường dẫn thư mục hoặc URL git+...); bỏ qua REPO/REF/token
#   GITHUB_TOKEN       token đọc repo private (cũng nhận GH_TOKEN, TDD_AGENT_TOKEN)
#
# Không có token thì script thử HTTPS với credential helper của git, rồi tới SSH (git@github.com).
set -eu

REPO="${TDD_AGENT_REPO:-blcvn/tdd-agentic-ai}"
REF="${TDD_AGENT_REF:-main}"
SOURCE="${TDD_AGENT_SOURCE:-}"
TOKEN="${TDD_AGENT_TOKEN:-${GITHUB_TOKEN:-${GH_TOKEN:-}}}"
PACKAGE="tdd-agent"

say() { printf '%s\n' "$*"; }
err() { printf 'tdd-agent: %s\n' "$*" >&2; }
have() { command -v "$1" >/dev/null 2>&1; }

# 1. Nền tảng. Windows thì chỉ hướng dẫn, vì script này là POSIX sh.
case "$(uname -s)" in
  Darwin|Linux) ;;
  MINGW*|MSYS*|CYGWIN*)
    err "trên Windows, mở PowerShell rồi chạy:"
    err "  powershell -c \"irm https://astral.sh/uv/install.ps1 | iex\""
    err "  uv tool install git+https://github.com/$REPO"
    exit 1
    ;;
  *) err "hệ điều hành '$(uname -s)' chưa được hỗ trợ."; exit 1 ;;
esac

# 2. uv. Cài nếu thiếu; bộ cài của uv đặt vào ~/.local/bin (hoặc $XDG_BIN_HOME, ~/.cargo/bin).
ensure_uv() {
  have uv && return 0
  have curl || { err "cần curl để cài uv."; exit 1; }
  say "Chưa có uv, đang cài uv (https://astral.sh/uv)..."
  curl -LsSf https://astral.sh/uv/install.sh | sh
  PATH="${XDG_BIN_HOME:-$HOME/.local/bin}:$HOME/.cargo/bin:$PATH"
  export PATH
  have uv || { err "đã cài uv nhưng chưa thấy trong PATH; mở shell mới rồi chạy lại."; exit 1; }
}

if [ "${1:-}" = "--uninstall" ]; then
  have uv || { err "không có uv nên không có gì để gỡ."; exit 0; }
  uv tool uninstall "$PACKAGE"
  say "Đã gỡ $PACKAGE. Dữ liệu dự án trong .tdd-agent/ được giữ nguyên."
  exit 0
fi

ensure_uv

# 3. Nguồn cài. Ưu tiên TDD_AGENT_SOURCE; không thì chọn đường tới GitHub mà git truy cập được.
if [ -z "$SOURCE" ]; then
  have git || { err "cần git để cài từ GitHub (hoặc đặt TDD_AGENT_SOURCE)."; exit 1; }
  https_url="https://github.com/$REPO.git"
  ssh_url="git@github.com:$REPO.git"

  # Token đi qua cấu hình git của tiến trình này (url.<...>.insteadOf) để không bị ghi vào
  # uv-receipt.toml cùng URL của gói.
  if [ -n "$TOKEN" ]; then
    GIT_CONFIG_COUNT=1
    GIT_CONFIG_KEY_0="url.https://x-access-token:${TOKEN}@github.com/.insteadOf"
    GIT_CONFIG_VALUE_0="https://github.com/"
    export GIT_CONFIG_COUNT GIT_CONFIG_KEY_0 GIT_CONFIG_VALUE_0
  fi

  reachable() { GIT_TERMINAL_PROMPT=0 git ls-remote --exit-code "$1" HEAD >/dev/null 2>&1; }

  if reachable "$https_url"; then
    url="$https_url"
  elif [ -z "$TOKEN" ] && reachable "$ssh_url"; then
    url="$ssh_url"
  else
    err "không đọc được $REPO trên GitHub."
    if [ -n "$TOKEN" ]; then
      err "token trong GITHUB_TOKEN/GH_TOKEN không có quyền đọc repo, hoặc đã hết hạn."
    else
      err "repo là private: đặt GITHUB_TOKEN (quyền đọc repo), hoặc cấu hình SSH key cho github.com."
    fi
    exit 1
  fi
  SOURCE="git+$url@$REF"
fi

# 4. Cài. --force để chạy lại script thì cập nhật lên bản mới nhất của REF.
say "Đang cài $PACKAGE từ $SOURCE ..."
uv tool install --force "$SOURCE"

# 5. PATH. uv đặt lệnh vào thư mục bin của tool.
bin_dir="$(uv tool dir --bin)"
[ -x "$bin_dir/tdd-agent" ] || { err "cài xong nhưng không thấy $bin_dir/tdd-agent."; exit 1; }

on_path=0
oldifs="$IFS"; IFS=:
for dir in $PATH; do
  [ "$dir" = "$bin_dir" ] && on_path=1
done
IFS="$oldifs"

say ""
"$bin_dir/tdd-agent" --help >/dev/null 2>&1 || { err "đã cài nhưng 'tdd-agent --help' lỗi."; exit 1; }
say "Đã cài: $bin_dir/tdd-agent"
if [ "$on_path" -eq 0 ]; then
  say ""
  say "$bin_dir chưa có trong PATH. Chạy:"
  say "  uv tool update-shell     # rồi mở lại terminal"
  say "hoặc tạm thời:"
  say "  export PATH=\"$bin_dir:\$PATH\""
fi

# 6. Các công cụ ngoài mà tdd-agent dùng; không bắt buộc để cài.
if ! have claude; then
  say ""
  say "Lưu ý: chưa thấy 'claude' (Claude Code). Cần để đọc văn xuôi bằng mô hình: https://claude.com/claude-code"
fi
if ! have openspec; then
  say ""
  say "Lưu ý: chưa thấy 'openspec'. Cần cho luồng 'tdd-agent change ...': npm i -g @fission-ai/openspec"
fi

say ""
say "Bắt đầu:  cd <dự-án> && tdd-agent init --urd docs/URD.md --srs docs/SRS.md --sad docs/SAD.md"
