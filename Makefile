.PHONY: default
default: build

dest = bin
GO_OS := $(shell go env GOOS)
GO_ARCH := $(shell go env GOARCH)

VERSION := $(shell git describe --tags)
VERSION_DEV := dev
VERSION_DIST := $(shell git describe --abbrev=0 --tags)

ifeq ($(OS),Windows_NT)
	WINDOWS_HIDE := -H=windowsgui
else
	WINDOWS_HIDE :=
endif


syso:
ifeq ($(OS),Windows_NT)
	go get -d github.com/akavel/rsrc
	go build -o bin/ github.com/akavel/rsrc
#	bin/rsrc -arch amd64 -ico ./web/icon/favicon.ico -o rsrc_windows_arm64.syso
	bin/rsrc -arch $(GO_ARCH) -ico ./web/icon/favicon.ico -o rsrc_$(GO_OS)_$(GO_ARCH).syso
	go mod tidy

	del .\$(dest)\rsrc* >nul 2>&1
endif


build: syso
	go build -ldflags "-w -s" -trimpath -o $(dest)/ ssh-client/
	go build -ldflags "-w -s -X 'main.VERSION=$(VERSION)' $(WINDOWS_HIDE)" -trimpath -o $(dest)/


dev: syso
	go build -ldflags "-w -s" -trimpath -o $(dest)/ ssh-client/
	go build -ldflags "-X 'main.VERSION=$(VERSION_DEV)'" -o $(dest)/


gox_build:
	go get -d github.com/mitchellh/gox
	go mod edit -replace github.com/mitchellh/gox=github.com/edp1096/gox@latest
	go build -mod=readonly -o $(dest)/ github.com/mitchellh/gox
	go mod tidy
	go env -w GOFLAGS=-trimpath

	$(dest)/gox -mod="readonly" -ldflags="-X main.VERSION=$(VERSION_DIST) -w -s" -output="$(dest)/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="windows/amd64 freebsd/amd64 linux/amd64 linux/arm linux/arm64" ./ssh-client
	$(dest)/gox -mod="readonly" -ldflags="-X main.VERSION=$(VERSION_DIST) -w -s -H=windowsgui" -output="$(dest)/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="windows/amd64 freebsd/amd64 linux/amd64 linux/arm linux/arm64"

gox_clean:
ifeq ($(OS),Windows_NT)
	del .\$(dest)\gox* /q /s >nul 2>&1
else
	rm $(dest)/gox*
endif

dist_build:
	go run ./tools/archiver -osarch "windows/amd64 freebsd/amd64 linux/amd64 linux/arm linux/arm64"

dist_clean:
ifeq ($(OS),Windows_NT)
	del .\$(dest)\ssh-client_* /q /s >nul 2>&1
	del .\$(dest)\ssh-manager_* /q /s >nul 2>&1
else
	rm $(dest)/ssh-client_*
	rm $(dest)/ssh-manager_*
endif

dist: clean syso gox_build gox_clean dist_build dist_clean

clean:
ifeq ($(OS),Windows_NT)
	-del .\$(dest)\*.exe /q /s >nul 2>&1
	-del .\$(dest)\ssh-manager* /q /s >nul 2>&1
	-del .\$(dest)\ssh-client* /q /s >nul 2>&1
	-del .\$(dest)\browser_data /q /s >nul 2>&1
	-del dist /q /s >nul 2>&1
	-del .\ssh-client\$(dest)\* /q /s >nul 2>&1
	-del .\rsrc_*.syso /q /s >nul 2>&1
	-del .\coverage.html /q /s >nul 2>&1
	-del .\coverage.out /q /s >nul 2>&1
else
	rm -rf $(dest)/*.exe
	rm -rf $(dest)/ssh-manager*
	rm -rf $(dest)/ssh-client*
	rm -rf $(dest)/browser_data
	rm -rf dist
	rm -rf ssh-client/$(dest)/*
	rm -f ./rsrc_*.syso
	rm -f ./coverage.html
	rm -f ./coverage.out
endif
