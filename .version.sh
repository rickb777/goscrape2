#!/bin/sh -e
cd "$(dirname $0)"
if [ -d .git ]; then
  git describe --tags --always --dirty 2>/dev/null
else
  echo dev
fi
