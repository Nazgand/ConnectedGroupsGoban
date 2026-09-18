#!/usr/bin/env bash
# Assumes NixOS
nix-shell -p go libGL pkg-config libx11 libxcursor libxi libxinerama libxrandr libxxf86vm libxkbcommon wayland --run "go run . $(printf ' %q' "$@")"
