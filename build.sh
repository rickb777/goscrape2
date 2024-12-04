#!/bin/sh -ex
cd "$(dirname $0)"

DATE=$(date '+%F')

if [ -d .git ]; then
  VERSION=$(git describe --tags --always --dirty 2>/dev/null)
else
  VERSION=dev
fi

rm -f goscrape2
go test ./...
go vet ./...
go install -ldflags "-s -X main.version=$VERSION -X main.date=$DATE" .
