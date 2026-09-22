#!/bin/sh
# Re-derive the Aura custom-property list from the published theme package and
# diff it against the table embedded in go/internal/tools/auradata.go, so drift
# shows up when a new Vaadin version ships.
#
#   sh scripts/derive-aura-properties.sh [version]
#
# Default version: the one the embedded table was measured from.
#
# What this can and cannot tell you:
#
#   - The *names* are fully derivable and are what this script checks. A name the
#     theme gained or dropped shows up as a diff against auraKnownProperties.
#   - The *write-safety* classification is NOT derivable from the CSS. Several
#     properties are computed from other properties and are still the documented,
#     intended override points (--aura-neutral-light, --aura-shadow-color, …), so
#     a "is it computed?" scan would flag exactly the properties an agent needs
#     most. That split lives in the Vaadin Aura reference pages, where a
#     read-only property carries a `Read-only` or `light-dark()` badge. When this
#     script reports new or changed names, classify them from those pages and
#     update auraReadOnlyProperties by hand.
set -e
cd "$(dirname "$0")/.." # repo root

VERSION="${1:-25.3.0-rc1}"
DATA="go/internal/tools/auradata.go"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Fetching @vaadin/aura@$VERSION…"
curl -fsSL "https://registry.npmjs.org/@vaadin/aura/-/aura-$VERSION.tgz" | tar xz -C "$TMP"
SRC="$TMP/package/src"

# Properties the theme defines…
grep -rhoE '^[[:space:]]+--aura-[a-z0-9-]+[[:space:]]*:' "$SRC"/*.css "$SRC"/components/*.css \
  | sed -E 's/^[[:space:]]*//; s/[[:space:]]*:$//' | sort -u > "$TMP/defined"

# …plus the ones it only reads through a fallback, which are customization hooks
# and therefore equally valid names to use.
grep -rhoE 'var\([[:space:]]*--aura-[a-z0-9-]+' "$SRC"/*.css "$SRC"/components/*.css \
  | sed -E 's/^var\([[:space:]]*//' | sort -u > "$TMP/read"
sort -u "$TMP/defined" "$TMP/read" > "$TMP/derived"

# The names embedded in the Go table (the auraKnownProperties block only).
sed -n '/^var auraKnownProperties/,/^}/p' "$DATA" \
  | grep -oE '"--aura-[a-z0-9-]+"' | tr -d '"' | sort -u > "$TMP/embedded"

echo "derived: $(wc -l < "$TMP/defined" | tr -d ' ') defined + \
$(comm -23 "$TMP/read" "$TMP/defined" | wc -l | tr -d ' ') read-only hooks = \
$(wc -l < "$TMP/derived" | tr -d ' ') names"
echo "embedded in $DATA: $(wc -l < "$TMP/embedded" | tr -d ' ') names"
echo

if diff -u "$TMP/embedded" "$TMP/derived" > "$TMP/diff"; then
  echo "✓ auraKnownProperties is up to date with @vaadin/aura@$VERSION."
  exit 0
fi

echo "✗ The embedded list differs from @vaadin/aura@$VERSION (- embedded, + derived):"
echo
sed '1,2d' "$TMP/diff"
echo
echo "Update auraKnownProperties in $DATA, then classify any new name as"
echo "read-only or customizable from its badge in the Vaadin Aura reference pages."
exit 1
