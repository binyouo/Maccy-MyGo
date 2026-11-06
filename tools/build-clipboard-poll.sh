#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p resources/darwin
clang -O2 -fobjc-arc -dynamiclib -arch arm64 -arch x86_64 \
  -mmacosx-version-min=11.0 -framework AppKit \
  -install_name @rpath/libmaccy-poll.dylib \
  tools/clipboard-poll.m -o resources/darwin/libmaccy-poll.dylib
codesign --force --sign - resources/darwin/libmaccy-poll.dylib
