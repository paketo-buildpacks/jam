#!/bin/bash
readonly PROGDIR="$(cd "$(dirname "${0}")" && pwd)"

echo "hello from the pre-packaging script"

for dir in some-os/some-arch some-other-os/some-other-arch; do
  mkdir -p "$PROGDIR/../$dir"

  echo "$dir/hello" > "$PROGDIR/../$dir/generated-file"

  chmod 644 "$PROGDIR/../$dir/generated-file"
done
