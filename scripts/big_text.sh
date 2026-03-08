#!/bin/bash

output="bigfile.txt"
head -c 1100000 /dev/urandom | base64 > "$output"

echo "Generated $(du -h "$output" | cut -f1) file: $output"
