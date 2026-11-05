#!/bin/bash
# bench.sh — end-to-end performance comparison: upstream Maccy (Swift) vs
# this Go port. It measures search cost on a shared corpus (both harnesses),
# then app-level metrics: launch, idle memory/CPU, clipboard capture latency,
# and UI search latency (Go side only; upstream has no automation interface).
#
# Safe-guards: the upstream app's real history (Storage.sqlite*) is copied to
# the bench dir and restored on exit; the currently running Go app is quit and
# relaunched at the end; the text clipboard is saved and restored.
#
# Usage: tools/bench/bench.sh [quick]
#   quick — fewer reps/launches for a smoke run
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BENCH=/tmp/maccy-bench
DATA="$BENCH/data"
OUT="$BENCH/out"
GO_BIN="$BENCH/MaccyGo"
SWIFT_BIN="$BENCH/searchbench-swift"
GO_STORE="$HOME/Library/Application Support/MaccyBench"
ORIG_APP="/Applications/Maccy.app/Contents/MacOS/Maccy"
ORIG_DB="$HOME/Library/Containers/org.p0deje.Maccy/Data/Library/Application Support/Maccy/Storage.sqlite"
REAL_GO_APP="$ROOT/build/darwin-arm64/Maccy.app"
SOCK="$BENCH/auto.sock"
AUTOMATION="${AUTOMATION:-/tmp/mygo-automation}"
FUSE_DIR="${FUSE_DIR:-/tmp/mygomaccy-fuse-reference/Fuse/Classes}"

QUICK=0
[ "${1:-}" = "quick" ] && QUICK=1
if [ "$QUICK" = 1 ]; then REPS=8; LAUNCHES=3; CAPN=10; else REPS=25; LAUNCHES=5; CAPN=20; fi

mkdir -p "$DATA" "$OUT" "$BENCH/backup"
now_ms() { python3 -c 'import time;print(int(time.time()*1000))'; }
say() { printf '\n\033[1m== %s\033[0m\n' "$*"; }

# ---------- build & data ----------
say "building benchmark binaries"
(cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$GO_BIN" .)
[ -f "$DATA/corpus.json" ] || (cd "$ROOT" && go run ./tools/bench/gen -out "$DATA")
if [ ! -x "$SWIFT_BIN" ]; then
  if [ ! -d "$FUSE_DIR" ]; then
    say "fetching fuse-swift 1.4.0"
    git clone https://github.com/krisk/fuse-swift "$BENCH/fuse-swift" &&
      git -C "$BENCH/fuse-swift" checkout 26ba868691b2d8b7bf2b1322951eb591be70ccca
    FUSE_DIR="$BENCH/fuse-swift/Fuse/Classes"
  fi
  swiftc -O -o "$SWIFT_BIN" "$ROOT/tools/bench/swiftsearch/main.swift" "$FUSE_DIR"/*.swift
fi

# ---------- microbenchmarks ----------
say "microbenchmark: search (shared corpus)"
for N in 200 2000 10000; do
  # Scale reps down as the corpus grows so total runtime stays sane; medians
  # remain stable at these counts.
  NREPS=$REPS
  [ "$N" -ge 2000 ] && NREPS=$((REPS > 8 ? 8 : REPS))
  [ "$N" -ge 10000 ] && NREPS=$((REPS > 3 ? 3 : REPS))
  (cd "$ROOT" && go run ./tools/bench/searchgo -corpus "$DATA/corpus.json" \
    -queries "$DATA/queries.json" -n $N -reps $NREPS) > "$OUT/go-search-$N.json" 2>/dev/null
  "$SWIFT_BIN" "$DATA/corpus.json" "$DATA/queries.json" $N $NREPS 3 \
    > "$OUT/swift-search-$N.json" 2>/dev/null
  echo "n=$N reps=$NREPS done"
done

# ---------- app-level prep ----------
say "preparing app-level benchmark"
pbpaste > "$BENCH/clipboard-backup.txt" 2>/dev/null || true

REAL_GO_PID=$(pgrep -f "$REAL_GO_APP/Contents/MacOS" | head -1 || true)
if [ -n "$REAL_GO_PID" ]; then
  kill "$REAL_GO_PID" 2>/dev/null || true
  for i in $(seq 1 50); do kill -0 "$REAL_GO_PID" 2>/dev/null || break; sleep 0.1; done
fi

ORIG_WAS_RUNNING=$(pgrep -f "$ORIG_APP" | head -1 || true)
if [ -n "$ORIG_WAS_RUNNING" ]; then
  kill "$ORIG_WAS_RUNNING" 2>/dev/null
  for i in $(seq 1 50); do pgrep -f "$ORIG_APP" >/dev/null || break; sleep 0.1; done
fi
cp "$ORIG_DB" "$ORIG_DB-shm" "$ORIG_DB-wal" "$BENCH/backup/" 2>/dev/null

rm -rf "$GO_STORE"
(cd "$ROOT" && go run ./tools/bench/seed -dir "$GO_STORE" -n 200 -corpus "$DATA/corpus.json" -imgfrac 0.3)

restore_all() {
  say "restoring state"
  pkill -f "$ORIG_APP" 2>/dev/null
  pkill -f "$GO_BIN" 2>/dev/null
  sleep 0.6
  cp "$BENCH/backup/Storage.sqlite" "$ORIG_DB" 2>/dev/null
  cp "$BENCH/backup/Storage.sqlite-shm" "$ORIG_DB-shm" 2>/dev/null
  cp "$BENCH/backup/Storage.sqlite-wal" "$ORIG_DB-wal" 2>/dev/null
  pbcopy < "$BENCH/clipboard-backup.txt" 2>/dev/null
  [ -d "$REAL_GO_APP" ] && open "$REAL_GO_APP" 2>/dev/null
  [ -n "$ORIG_WAS_RUNNING" ] && open "/Applications/Maccy.app" 2>/dev/null
}
trap restore_all EXIT

cpu_time_s() { ps -o time= -p "$1" 2>/dev/null | awk -F: '{n=NF; s=0; m=1; for(i=n;i>=1;i--){s+=$i*m; m*=60} print s}'; }
mem_kb() { ps -o rss= -p "$1" 2>/dev/null | tr -d ' '; }
footprint_kb() {
  /usr/bin/footprint -p "$1" 2>/dev/null | awk '
    /phys_footprint:/{ v=$2; u=$3; gsub(/[^0-9.]/,"",v); if(u=="GB")v*=1048576; else if(u=="MB")v*=1024; print int(v); exit }'
}

# launch_time <pid> <t0_ms> <probe...>
# Prints two numbers: ms until probe first succeeded, ms until the process
# stayed below ~2% CPU (delta of `ps time`) for 3 consecutive 60ms samples.
launch_time() {
  local pid=$1 t0=$2; shift 2
  local quiet=0 t_first="" cprev
  cprev=$(cpu_time_s $pid)
  for i in $(seq 1 350); do
    kill -0 "$pid" 2>/dev/null || { echo "-1 -1"; return; }
    sleep 0.06
    if [ -z "$t_first" ] && "$@" >/dev/null 2>&1; then t_first=$(( $(now_ms) - t0 )); fi
    local cnow busy
    cnow=$(cpu_time_s $pid)
    busy=$(python3 -c "print(0 if ($cnow-$cprev) < 0.02 else 1)" 2>/dev/null || echo 1)
    cprev=$cnow
    if [ "$busy" = 0 ]; then
      quiet=$((quiet+1))
      if [ "$quiet" -ge 3 ] && [ -n "$t_first" ]; then
        echo "$t_first $(( $(now_ms) - t0 ))"; return
      fi
    else quiet=0; fi
  done
  echo "${t_first:--1} $(( $(now_ms) - t0 ))"
}

say "launch + idle: upstream Maccy (real 200-item / 1.8GB history), x$LAUNCHES"
O_T1=(); O_T2=(); O_RSS=(); O_FP=(); O_CPU=()
for i in $(seq 1 $LAUNCHES); do
  T0=$(now_ms)
  "$ORIG_APP" >/dev/null 2>&1 &
  PID=$!
  T=($(launch_time $PID $T0 python3 "$ROOT/tools/bench/fdprobe.py" $PID Storage.sqlite))
  O_T1+=(${T[0]}); O_T2+=(${T[1]})
  sleep 4
  O_RSS+=($(mem_kb $PID)); O_FP+=($(footprint_kb $PID))
  C0=$(cpu_time_s $PID); sleep 8; C1=$(cpu_time_s $PID)
  O_CPU+=($(python3 -c "print(round(($C1-$C0)/8*100,2))"))
  kill $PID 2>/dev/null; wait $PID 2>/dev/null
done
echo "upstream t_dbopen_ms=${O_T1[*]} t_quiet_ms=${O_T2[*]} rss_kb=${O_RSS[*]} footprint_kb=${O_FP[*]} idle_cpu_pct=${O_CPU[*]}" | tee "$OUT/upstream-launch.txt"

say "launch + idle: Go port (seeded 200-item store), x$LAUNCHES"
G_T1=(); G_T2=(); G_RSS=(); G_FP=(); G_CPU=()
for i in $(seq 1 $LAUNCHES); do
  rm -f "$SOCK"
  T0=$(now_ms)
  MACCY_NAME=MaccyBench MYGO_AUTOMATION="$SOCK" "$GO_BIN" >/dev/null 2>&1 &
  PID=$!
  T=($(launch_time $PID $T0 bash -c "[ -S '$SOCK' ]"))
  G_T1+=(${T[0]}); G_T2+=(${T[1]})
  sleep 4
  G_RSS+=($(mem_kb $PID)); G_FP+=($(footprint_kb $PID))
  C0=$(cpu_time_s $PID); sleep 8; C1=$(cpu_time_s $PID)
  G_CPU+=($(python3 -c "print(round(($C1-$C0)/8*100,2))"))
  kill $PID 2>/dev/null; wait $PID 2>/dev/null
done
echo "go t_sock_ms=${G_T1[*]} t_quiet_ms=${G_T2[*]} rss_kb=${G_RSS[*]} footprint_kb=${G_FP[*]} idle_cpu_pct=${G_CPU[*]}" | tee "$OUT/go-launch.txt"

# ---------- capture latency ----------
say "clipboard capture latency (both apps live, $CAPN writes)"
"$ORIG_APP" >/dev/null 2>&1 &
ORIG_PID=$!
rm -f "$SOCK"
MACCY_NAME=MaccyBench MYGO_AUTOMATION="$SOCK" "$GO_BIN" >/dev/null 2>&1 &
GO_PID=$!
sleep 4

O_LAG=(); G_LAG=()
for i in $(seq 1 $CAPN); do
  # Random jitter so writes don't phase-lock to either app's 500ms poll tick;
  # without it, whichever app is slower becomes the loop's clock and its lag
  # clusters near one full interval.
  sleep "$(python3 -c 'import random;print(round(random.uniform(0.05,0.95),2))')"
  M="MACCYBENCH-$(now_ms)-$i"
  T0=$(now_ms)
  printf %s "$M" | pbcopy
  O_SEEN=""; G_SEEN=""
  for k in $(seq 1 120); do
    if [ -z "$O_SEEN" ]; then
      C=$(sqlite3 "file:$ORIG_DB?mode=ro" "SELECT COUNT(*) FROM ZHISTORYITEM WHERE ZTITLE LIKE '%$M%'" 2>/dev/null)
      [ "${C:-0}" -ge 1 ] && O_SEEN=$(( $(now_ms) - T0 ))
    fi
    if [ -z "$G_SEEN" ] && grep -q "$M" "$GO_STORE/history.json" 2>/dev/null; then
      G_SEEN=$(( $(now_ms) - T0 ))
    fi
    [ -n "$O_SEEN" ] && [ -n "$G_SEEN" ] && break
    sleep 0.04
  done
  O_LAG+=(${O_SEEN:--1}); G_LAG+=(${G_SEEN:--1})
done
echo "capture-lag-ms upstream=${O_LAG[*]} go=${G_LAG[*]}" | tee "$OUT/capture.txt"

# ---------- Go UI search latency ----------
say "UI search latency (Go port, via automation)"
kill $GO_PID 2>/dev/null; wait $GO_PID 2>/dev/null
rm -f "$SOCK"
MACCY_NAME=MaccyBench MYGO_AUTOMATION="$SOCK" MACCY_AUTOSHOW=1 "$GO_BIN" >/dev/null 2>&1 &
GO_PID=$!
sleep 3.5
if "$AUTOMATION" open --socket "$SOCK" > "$OUT/auto-open.txt" 2>&1; then
  "$AUTOMATION" windows >> "$OUT/auto-open.txt" 2>&1
  "$AUTOMATION" snapshot > "$OUT/auto-snapshot.txt" 2>&1
  for PAIR in "needle-alpha-42|needle-alpha" "needle|needle" "剪贴板历史|剪贴板"; do
    Q="${PAIR%%|*}"; EXPECT="${PAIR##*|}"
    "$AUTOMATION" key ctrl-u >/dev/null 2>&1 || true; sleep 0.2
    T0=$(now_ms)
    "$AUTOMATION" type "$Q" >/dev/null 2>&1
    "$AUTOMATION" wait --text "$EXPECT" >/dev/null 2>&1
    T1=$(now_ms)
    echo "query=$Q ui_settle_ms=$((T1-T0))" | tee -a "$OUT/ui-search.txt"
  done
  "$AUTOMATION" close >/dev/null 2>&1 || true
else
  echo "automation session failed" | tee "$OUT/ui-search.txt"
fi

# ---------- sizes ----------
say "binary & bundle sizes"
{
  echo "upstream binary: $(stat -f%z "$ORIG_APP") bytes"
  echo "upstream bundle: $(du -sh /Applications/Maccy.app | cut -f1)"
  echo "go binary:       $(stat -f%z "$GO_BIN") bytes"
  [ -d "$REAL_GO_APP" ] && echo "go bundle:       $(du -sh "$REAL_GO_APP" | cut -f1)"
} | tee "$OUT/sizes.txt"

say "done — raw results in $OUT"
