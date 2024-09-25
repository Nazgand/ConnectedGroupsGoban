GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build -o ConnectedGroupsGoban.windows.amd64.exe
GOOS=linux GOARCH=amd64 go build -o ConnectedGroupsGoban.linux.amd64
