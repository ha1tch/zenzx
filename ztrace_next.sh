#!/bin/sh
# ztrace_next.sh - trace the Next boot chain from reset with the trace harness (T-36).
#
# Usage: sh ztrace_next.sh [steps]      default 5000000 instructions
#
# Uses the same three files as run_next_divmmc.sh, from ./thirdparty or NEXT_FILES. The
# trace goes to ./next-trace.log (ZTRACE_OUT names another file) and grows by about 75
# bytes per instruction: 5 million steps is about 375 MB, 50 million about 3.7 GB.
# The harness steps the CPU itself and does not carry out a reset asked for through
# NextREG 0x02, so the trace ends where the firmware hands over to the machine.

cd "$(dirname "$0")" || exit 1
DIR=${NEXT_FILES:-./thirdparty}

ZTRACE_OUT=${ZTRACE_OUT:-./next-trace.log} \
ZTRACE_MODEL=next \
ZTRACE_SETUP=boot-only \
ZTRACE_FROM_RESET=1 \
ZTRACE_ENTRY=0000 \
ZTRACE_MAX_STEPS=${1:-5000000} \
ZTRACE_M1LOG=1 \
ZTRACE_BOOTROM="$DIR/nextboot.rom" \
ZTRACE_DIVMMC_ROM="$DIR/enNxtmmc.rom" \
ZTRACE_SDCARD="$DIR/next-card.img" \
go test -tags headless -run TestTraceHarness -v .
