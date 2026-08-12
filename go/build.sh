#!/bin/sh
# Cross-compile the vaadin-agent-tools CLI for every supported OS/arch and place
# the binaries where the plugin launcher expects them (../bin/platform).
#
# No runtime is bundled or hunted for — each binary is fully self-contained
# (CGO disabled, symbols stripped). The plugin's bin/vaadin-agent-tools selector
# picks the right one by uname; it never looks outside the plugin directory.
set -e
cd "$(dirname "$0")" # go/

OUT="../bin/platform"
mkdir -p "$OUT"

export CGO_ENABLED=0
LDFLAGS="-s -w" # strip symbol/debug info → smaller, cleaner binaries (no UPX!)

build() { # os arch outfile
  echo "  $1/$2"
  GOOS="$1" GOARCH="$2" go build -trimpath -ldflags "$LDFLAGS" -o "$OUT/$3" .
}

echo "Building vaadin-agent-tools:"
build linux   amd64 vaadin-agent-tools-linux-amd64
build linux   arm64 vaadin-agent-tools-linux-arm64
build windows amd64 vaadin-agent-tools-windows-amd64.exe

# macOS: combine amd64 + arm64 into one universal binary when lipo is available,
# otherwise ship both and let the selector pick.
TMP="$(mktemp -d)"
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$LDFLAGS" -o "$TMP/amd64" .
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$LDFLAGS" -o "$TMP/arm64" .
if command -v lipo >/dev/null 2>&1; then
  echo "  darwin/universal"
  lipo -create -output "$OUT/vaadin-agent-tools-darwin" "$TMP/amd64" "$TMP/arm64"
else
  echo "  darwin/amd64 + darwin/arm64 (no lipo; shipping both)"
  cp "$TMP/amd64" "$OUT/vaadin-agent-tools-darwin-amd64"
  cp "$TMP/arm64" "$OUT/vaadin-agent-tools-darwin-arm64"
fi
rm -rf "$TMP"

echo "Done → $OUT"
ls -la "$OUT"
