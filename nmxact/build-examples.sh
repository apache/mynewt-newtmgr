#!/bin/sh

read -r -d '' exdirs <<-'EOS'
    ble_loop
    ble_plain
    serial_plain
EOS

rc=0

for d in $exdirs
do
    if ! (cd example/"$d" && go build)
    then
        printf '*** Failed to build %s\n' "$d" >&2
        rc=1
    fi
done

exit "$rc"
