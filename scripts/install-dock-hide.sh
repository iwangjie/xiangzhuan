#!/bin/sh
# Adds the dock-hide/rest-lock dylib to a packaged 香篆.app. The dylib follows
# `hideDockIcon`: Accessory (menu bar only) when on, Regular (Dock icon) when
# off, and enforces the rest lock while storage says `restLock` is "1".
set -eu

APP="${1:-.nsh/out/香篆.app}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PLIST="$APP/Contents/Info.plist"
RES="$APP/Contents/Resources"
DYLD_PATH='@executable_path/../Resources/dock_hide.dylib'

[ -d "$APP" ] || { echo "no app at $APP" >&2; exit 1; }
mkdir -p "$RES"

clang -dynamiclib -O2 -fobjc-arc -framework AppKit \
  -o "$RES/dock_hide.dylib" "$ROOT/native/dock_hide.m"

/usr/libexec/PlistBuddy -c "Delete :LSEnvironment" "$PLIST" >/dev/null 2>&1 || true
/usr/libexec/PlistBuddy \
  -c "Add :LSEnvironment dict" \
  -c "Add :LSEnvironment:DYLD_INSERT_LIBRARIES string $DYLD_PATH" \
  "$PLIST"

codesign --force -s - "$RES/dock_hide.dylib"
codesign --force --deep -s - "$APP"

echo "dock-hide + rest-lock dylib installed in $APP"
