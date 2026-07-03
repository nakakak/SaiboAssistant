#!/usr/bin/env bash
# 赛搏小小助手 Release 打包（与 .github/workflows/release.yml 产物一致）
#
# 用法:
#   ./scripts/package-release.sh              # 本机单包 → dist/
#   ./scripts/package-release.sh v0.2.15
#   ./scripts/package-release.sh v0.2.15 --all
set -euo pipefail
cd "$(dirname "$0")/.."
VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo "v0.0.0-local")}"
if [[ "${2:-}" == "--all" ]]; then
  ALL=1
elif [[ "$VERSION" == "--all" ]]; then
  ALL=1
  VERSION="$(git describe --tags --always 2>/dev/null || echo "v0.0.0-local")"
else
  ALL=0
fi
OUT="dist"
rm -rf "$OUT"
mkdir -p "$OUT"

build_one() {
  local goos="$1" goarch="$2"
  local archive="openclaw-connector_${VERSION}_${goos}_${goarch}"
  export GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=1
  if [[ "$goos" == "windows" ]]; then
    go build -trimpath -ldflags="-s -w" -o "SaiboAssistant.exe" ./cmd/openclaw-connector
    mkdir -p "$archive"
    cp SaiboAssistant.exe config.example.yaml README.md iconai.jpg "$archive/"
    cp SaiboAssistant.exe "${OUT}/SaiboAssistant-Windows-x64.exe"
    zip -r "${OUT}/${archive}.zip" "$archive"
    rm -rf "$archive" SaiboAssistant.exe
  else
    go build -trimpath -ldflags="-s -w" -o SaiboAssistant ./cmd/openclaw-connector
    mkdir -p "$archive"
    cp SaiboAssistant config.example.yaml README.md iconai.jpg "$archive/"
    case "${goos}-${goarch}" in
      darwin-arm64)  cp SaiboAssistant "${OUT}/SaiboAssistant-macOS-arm64" ;;
      darwin-amd64)  cp SaiboAssistant "${OUT}/SaiboAssistant-macOS-x64" ;;
      linux-amd64)   cp SaiboAssistant "${OUT}/SaiboAssistant-Linux-x64" ;;
      linux-arm64)   cp SaiboAssistant "${OUT}/SaiboAssistant-Linux-arm64" ;;
    esac
    tar czf "${OUT}/${archive}.tar.gz" "$archive"
    rm -rf "$archive" SaiboAssistant
  fi
  echo "  built ${OUT}/${archive}.*"
}

host_platform() {
  local goos goarch
  case "$(uname -s)" in
    Darwin) goos=darwin ;;
    Linux) goos=linux ;;
    MINGW*|MSYS*|CYGWIN*) goos=windows ;;
    *) echo "unsupported host OS: $(uname -s)" >&2; exit 1 ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) goarch=amd64 ;;
    arm64|aarch64) goarch=arm64 ;;
    *) echo "unsupported host arch: $(uname -m)" >&2; exit 1 ;;
  esac
  build_one "$goos" "$goarch"
}

echo "packaging SaiboAssistant ${VERSION} (CGO_ENABLED=1)"
if [[ "$ALL" -eq 1 ]]; then
  build_one linux amd64
  build_one linux arm64
  build_one darwin amd64
  build_one darwin arm64
  build_one windows amd64
else
  host_platform
fi

(
  cd "$OUT"
  shopt -s nullglob
  arr=(openclaw-connector_*.tar.gz openclaw-connector_*.zip SaiboAssistant-*)
  if [[ ${#arr[@]} -gt 0 ]]; then
    sha256sum "${arr[@]}" 2>/dev/null | sort > checksums-saibo-sha256.txt || \
      shasum -a 256 "${arr[@]}" | sort > checksums-saibo-sha256.txt
  fi
)
echo "Done. Archives under ${OUT}/"
ls -la "$OUT"
