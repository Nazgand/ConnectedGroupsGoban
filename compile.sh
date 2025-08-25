# Assumes some form of Debian

sudo apt install golang libx11-dev libxrandr-dev libxcursor-dev libglx-dev libxinerama-dev build-essential libgl1-mesa-dev libglu1-mesa-dev freeglut3-dev mesa-common-dev libxi-dev gcc-mingw-w64-x86-64 libxxf86vm-dev -y
go install fyne.io/tools/cmd/fyne@latest
go get fyne.io/fyne/v2@latest
go mod tidy

if ! echo "$PATH" | grep -Eq "(^|:)$(go env GOPATH)/bin(:|$)"; then
  export PATH="$PATH:$(go env GOPATH)/bin"
fi

GOOS=linux GOARCH=amd64 go build -o ConnectedGroupsGoban.linux.amd64
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -o ConnectedGroupsGoban.windows.amd64.exe
# ANDROID_NDK_HOME=~/android-ndk-r29-beta3 fyne package --os android --app-id com.nazgand.connectedgroupsgoban --app-id Icon.png
