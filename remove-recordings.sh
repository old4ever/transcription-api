#!/bin/bash

while true; do $(ls -la *.wav | awk '{print $7}' | head -n -4 | tr "\n" " " | xargs rm -f) && sleep 2; done
