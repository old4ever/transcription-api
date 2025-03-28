#!/usr/bin/env bash

INPUT=alsa_input.usb-SteelSeries_SteelSeries_Arctis_1_Wireless-00.mono-fallback

while true; do pactl set-source-volume "$INPUT" 120% && sleep 1; done
