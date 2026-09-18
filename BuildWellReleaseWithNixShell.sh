#!/usr/bin/env bash
# Assumes NixOS
nix-shell -p go libGL pkg-config libx11 libxcursor libxi libxinerama libxrandr libxxf86vm libxkbcommon wayland --run "go install fyne.io/tools/cmd/fyne@latest && fyne package -os linux -icon ConnectedGroupsGobanIcon.png -release"
