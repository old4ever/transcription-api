#!/bin/bash

while true; do $(ls *.wav | head -n -4 | tr "\n" " " | xargs rm -f) && sleep 2; done
