#!/bin/bash
# Build (and optionally flash) an unmodified upstream Mozzi example sketch
# for an M5Stack CoreS3, to compare it with this port's
# example at the same path under examples/, built with -tags cores3, on the
# same board.
#
#   tools/cores3-original/build.sh 01.Basics/Sinewave            # build only
#   tools/cores3-original/build.sh 01.Basics/Sinewave /dev/ttyACM0   # build and flash
#
# The sketch is copied into a scratch directory, with mozzi_cores3.h
# included at the top (I2S pins, 16384Hz audio rate, speaker amp setup) and
# cores3EnableSpeaker() called at the start of setup(); the original file
# is left untouched.
#
# Needs an arduino-cli with the esp32:esp32 core (3.x) installed. Set
# ARDUINO_CLI to use one that isn't on PATH, e.g. a wrapper script that
# points arduino-cli at its own data directory. Upstream Mozzi is taken from
# MOZZI_DIR (default: the ../Mozzi checkout next to this port). Mozzi 2.x also
# needs the FixMath library (https://github.com/tomcombriat/FixMath): either
# install it into arduino-cli's libraries, or list its directory (and any
# other library directories, colon-separated) in LIBRARIES.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MOZZI_DIR="${MOZZI_DIR:-$(cd "$HERE/../../.." && pwd)/Mozzi}"
ARDUINO_CLI="${ARDUINO_CLI:-arduino-cli}"
FQBN="${FQBN:-esp32:esp32:m5stack_cores3}"
WORK="${WORK:-${TMPDIR:-/tmp}/mozzi-cores3-original}"

if [ $# -lt 1 ]; then
  echo "usage: $0 <example, e.g. 01.Basics/Sinewave> [port]" >&2
  exit 2
fi
example="$1"
port="${2:-}"
src="$MOZZI_DIR/examples/$example"
name="$(basename "$example")"
if [ ! -f "$src/$name.ino" ]; then
  echo "no sketch at $src/$name.ino" >&2
  exit 1
fi

sketch="$WORK/sketches/$name"
rm -rf "$sketch"
mkdir -p "$sketch"
cp "$src"/* "$sketch/"
cp "$HERE/mozzi_cores3.h" "$sketch/"
{
  echo '#include "mozzi_cores3.h" // added by tools/cores3-original/build.sh'
  sed -E 's/^(void[[:space:]]+setup[[:space:]]*\([[:space:]]*\)[[:space:]]*\{)/\1 cores3EnableSpeaker(); \/* added by build.sh *\//' "$src/$name.ino"
} > "$sketch/$name.ino"
if ! grep -q 'cores3EnableSpeaker(); /\* added' "$sketch/$name.ino"; then
  echo "couldn't find 'void setup() {' in $name.ino to add the speaker setup to" >&2
  exit 1
fi

args=(compile --fqbn "$FQBN" --library "$MOZZI_DIR" --build-path "$WORK/build/$name")
IFS=: read -r -a extra_libs <<< "${LIBRARIES:-}"
for lib in "${extra_libs[@]}"; do
  [ -n "$lib" ] && args+=(--library "$lib")
done
args+=("$sketch")
if [ -n "$port" ]; then
  args+=(--upload -p "$port")
fi
"$ARDUINO_CLI" "${args[@]}"
