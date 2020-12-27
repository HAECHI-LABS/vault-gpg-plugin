#!/usr/bin/env bash

set -e

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
cd -P "$( dirname "$SOURCE" )/.."

while IFS= read -r -d '' platform
do
    osarch=$(basename "$platform")

    pushd "$platform" >/dev/null 2>&1
    sha256sum -- * > "$osarch".sha256sum
    echo "--> ${osarch}.sha256sum"
    popd >/dev/null 2>&1
done <   <(find ./pkg -mindepth 1 -maxdepth 1 -type d -print0)