GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -o ConnectedGroupsGoban.windows.amd64.exe
ANDROID_NDK_HOME=~/android-ndk-r21e fyne package -os android -appID com.nazgand.connectedgroupsgoban -icon Icon.png
GOOS=linux GOARCH=amd64 go build -o ConnectedGroupsGoban.linux.amd64
