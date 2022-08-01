#!/bin/bash/
./ipfs daemon --enable-gc > daemon.txt 2>&1 &
sleep 10
./ipfs log level metric warn
#readelf -d ./extractServer | grep 'NEEDED'
./extractServer