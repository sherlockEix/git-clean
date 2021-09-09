all: win linux mac

win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -o dist/git-clean-win.exe main.go

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/git-clean-linux main.go

mac:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/git-clean-mac main.go