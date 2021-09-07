all: win linux mac

win:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64  go build -o dist/windows/git-clean main.go

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/linux/git-clean main.go

mac:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o dist/mac/git-clean main.go