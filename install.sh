#!/usr/bin/env sh
# Installer dbprobe — mengunduh binary rilis terbaru sesuai OS/arch.
#
#   curl -fsSL https://github.com/OWNER/dbprobe/releases/latest/download/install.sh | sh
#
# Variabel opsional:
#   DBPROBE_REPO=OWNER/dbprobe   # ganti repo sumber
#   DBPROBE_PREFIX=/usr/local/bin
set -eu

REPO="${DBPROBE_REPO:-OWNER/dbprobe}"
BIN="dbprobe"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux)  os=linux ;;
  darwin) os=darwin ;;
  *) echo "OS tidak didukung: $os" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)   arch=amd64 ;;
  arm64|aarch64)  arch=arm64 ;;
  *) echo "Arsitektur tidak didukung: $arch" >&2; exit 1 ;;
esac

file="dbprobe_${os}_${arch}"
url="https://github.com/${REPO}/releases/latest/download/${file}"

echo "OS/arch  : ${os}/${arch}"
echo "Mengunduh: ${url}"
curl -fsSL "$url" -o "$BIN"
chmod +x "$BIN"

dest="${DBPROBE_PREFIX:-/usr/local/bin}"
if [ -d "$dest" ] && [ -w "$dest" ]; then
  mv "$BIN" "$dest/$BIN"
  echo "Terpasang di ${dest}/${BIN}"
else
  echo "Tidak bisa menulis ke ${dest}."
  echo "Jalankan ulang dengan sudo, set DBPROBE_PREFIX, atau pindahkan manual:"
  echo "  sudo mv ./${BIN} ${dest}/${BIN}"
fi
