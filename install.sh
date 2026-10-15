#!/usr/bin/env sh
# Installer dbprobe (repo: warhamhayat/db-utilitas-tools).
#
#   curl -fsSL https://github.com/warhamhayat/db-utilitas-tools/releases/latest/download/install.sh | sh
#
# Cara kerja:
#   1. Deteksi OS + arsitektur.
#   2. Unduh binary prebuilt yang cocok (tersedia: linux/amd64, windows/amd64).
#   3. Kalau tidak ada prebuilt untuk platform ini (mis. macOS/arm64), coba build
#      dari source (bila Go terpasang). Kalau Go tidak ada, tampilkan instruksi.
#
# Variabel opsional:
#   DBPROBE_REPO=warhamhayat/db-utilitas-tools
#   DBPROBE_PREFIX=/usr/local/bin
set -eu

REPO="${DBPROBE_REPO:-warhamhayat/db-utilitas-tools}"
BIN="dbprobe"
BASE="https://github.com/${REPO}/releases/latest/download"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux|darwin) ;;
  *) echo "OS tidak dikenal: $os" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "Arsitektur tidak dikenal: $arch" >&2; exit 1 ;;
esac

asset="dbprobe_${os}_${arch}"
echo "OS/arch : ${os}/${arch}"

build_from_source() {
  if ! command -v go >/dev/null 2>&1; then
    echo "Go tidak terpasang, jadi tidak bisa build dari source." >&2
    echo "Unduh manual (platform lain) di: https://github.com/${REPO}/releases" >&2
    exit 1
  fi

  echo "Membangun dari source (butuh Go)..."
  tmp="$(mktemp -d)"
  git clone --depth 1 "https://github.com/${REPO}.git" "$tmp/src"

  # Dukung dua layout: repo root = isi folder DB, atau ada subfolder DB/.
  if [ -d "$tmp/src/cmd/dbprobe" ]; then
    srcdir="$tmp/src"
  elif [ -d "$tmp/src/DB/cmd/dbprobe" ]; then
    srcdir="$tmp/src/DB"
  else
    echo "Tidak menemukan package ./cmd/dbprobe di repo." >&2
    exit 1
  fi

  ( cd "$srcdir" && go build -trimpath -ldflags="-s -w" -o "${tmp}/${BIN}" ./cmd/dbprobe )
  mv "${tmp}/${BIN}" "./${BIN}"
}

echo "Mengunduh: ${BASE}/${asset}"
if curl -fsSL "${BASE}/${asset}" -o "$BIN"; then
  echo "Terunduh ${asset}."
else
  echo "Prebuilt ${asset} tidak tersedia; fallback ke build dari source."
  build_from_source
fi
chmod +x "$BIN" 2>/dev/null || true

dest="${DBPROBE_PREFIX:-/usr/local/bin}"
if [ -d "$dest" ] && [ -w "$dest" ]; then
  mv "$BIN" "$dest/$BIN"
  echo "Terpasang di ${dest}/${BIN}"
  "${dest}/${BIN}" --version 2>/dev/null || true
else
  echo "Tidak bisa menulis ke ${dest}. Binary ada di ./${BIN}."
  echo "Pindahkan manual, mis.: sudo mv ./${BIN} ${dest}/${BIN}"
fi
