.PHONY: default
default: build

dest = bin
GO_OS := $(shell go env GOOS)
GO_ARCH := $(shell go env GOARCH)

VERSION := $(shell git describe --tags)
VERSION_DEV := dev
VERSION_DIST := $(shell git describe --abbrev=0 --tags)

ifeq ($(GO_OS),windows)
	WINDOWS_HIDE := -H=windowsgui
else
	WINDOWS_HIDE :=
endif


syso:
ifeq ($(GO_OS),windows)
	go get -d github.com/akavel/rsrc
	go build -o bin/ github.com/akavel/rsrc
#	bin/rsrc -arch amd64 -ico ./html/icon/favicon.ico -o rsrc_windows_arm64.syso
	bin/rsrc -arch $(GO_ARCH) -ico ./html/icon/favicon.ico -o rsrc_$(GO_OS)_$(GO_ARCH).syso
	go mod tidy
	rm $(dest)/rsrc*
endif


build: syso
	go build -ldflags "-w -s" -trimpath -o $(dest)/ ssh-client/
	go build -ldflags "-w -s -X 'main.VERSION=$(VERSION)' $(WINDOWS_HIDE)" -trimpath -o $(dest)/


dev: syso
	go build -ldflags "-w -s" -trimpath -o $(dest)/ ssh-client/
	go build -ldflags "-X 'main.VERSION=$(VERSION_DEV)'" -o $(dest)/


dist: clean syso
	go get -d github.com/mitchellh/gox
	go mod edit -replace github.com/mitchellh/gox=github.com/edp1096/gox@latest
	go build -mod=readonly -o $(dest)/ github.com/mitchellh/gox
	go mod tidy
	go env -w GOFLAGS=-trimpath

	$(dest)/gox -mod="readonly" -ldflags="-X main.VERSION=$(VERSION_DIST) -w -s" -output="$(dest)/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="windows/amd64 freebsd/amd64 linux/amd64 linux/arm linux/arm64" ./ssh-client
	$(dest)/gox -mod="readonly" -ldflags="-X main.VERSION=$(VERSION_DIST) -w -s -H=windowsgui" -output="$(dest)/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="windows/amd64 freebsd/amd64 linux/amd64 linux/arm linux/arm64"
	rm $(dest)/gox*

	go run ./builder/archiver -osarch "windows/amd64 freebsd/amd64 linux/amd64 linux/arm linux/arm64"
	rm $(dest)/ssh-client_*
	rm $(dest)/ssh-manager_*


clean:
	rm -rf $(dest)/*.exe
	rm -rf $(dest)/ssh-manager*
	rm -rf $(dest)/ssh-client*
	rm -rf $(dest)/browser_data
	rm -rf dist
	rm -rf ssh-client/$(dest)/*
	rm -f ./rsrc_*.syso
	rm -f ./coverage.html
	rm -f ./coverage.out
