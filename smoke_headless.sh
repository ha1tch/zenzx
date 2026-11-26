#!/bin/sh
# Release gate: boot a 48K and a TS2068 in the headless binary and confirm
# each produces a valid PNG screenshot. Requires ./zenzx-headless (build step).
set -e
cd "$(dirname "$0")"
[ -x ./zenzx-headless ] || { echo "zenzx-headless missing -- run the build step first"; exit 1; }
SHOT_DIR=$(mktemp -d)
trap 'rm -rf "$SHOT_DIR"' EXIT

boot_check() {
    model="$1"
    ./zenzx-headless -model="$model" -romdir=./rom -frames=100 \
        -shot-dir="$SHOT_DIR" -shot-prefix="smoke-$model" -quiet >/dev/null 2>&1 \
        || { echo "headless smoke run failed for -model=$model"; exit 1; }
    SHOT="$SHOT_DIR/smoke-$model-frame000100.png"
    [ -f "$SHOT" ] || { echo "smoke test for -model=$model produced no screenshot"; exit 1; }
    MAGIC=$(head -c 8 "$SHOT" | od -An -tx1 | tr -d ' \n')
    [ "$MAGIC" = "89504e470d0a1a0a" ] || { echo "screenshot for -model=$model is not a valid PNG ($MAGIC)"; exit 1; }
    echo "Booted $model and produced a valid PNG screenshot"
}

boot_check 48k
boot_check ts2068
