# Assumes some form of Debian

sudo apt install golang libx11-dev libxrandr-dev libxcursor-dev libglx-dev libxinerama-dev build-essential libgl1-mesa-dev libglu1-mesa-dev freeglut3-dev mesa-common-dev libxi-dev gcc-mingw-w64-x86-64 libxxf86vm-dev -y
go install fyne.io/tools/cmd/fyne@latest
# go get fyne.io/fyne/v2@latest
go mod tidy

~/go/bin/fyne package -os linux -icon ConnectedGroupsGobanIcon.png
