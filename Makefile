dest = bin
GO_OS := $(shell go env GOOS)
GO_ARCH := $(shell go env GOARCH)

WINDOWS_HIDE := -H=windowsgui
#ifeq ($(GO_OS),windows)
#	WINDOWS_HIDE := -H=windowsgui
#else
#	WINDOWS_HIDE :=
#endif

ifndef version
#	version = 0.0.1
	version = dev
endif

build:
	go build -ldflags "-w -s" -trimpath -o $(dest)/ ssh-client/
	go build -ldflags "-w -s $(WINDOWS_HIDE)" -trimpath -o $(dest)/
#	go build -ldflags "-w -s" -trimpath -o $(dest)/

dist: clean
	go get -d github.com/mitchellh/gox
	go mod edit -replace github.com/mitchellh/gox=github.com/edp1096/gox@latest
	go build -mod=readonly -o $(dest)/ github.com/mitchellh/gox
	go mod tidy
	go env -w GOFLAGS=-trimpath

	$(dest)/gox -mod="readonly" -ldflags="-X main.Version=$(version) -w -s" -output="$(dest)/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="windows/amd64 linux/amd64 linux/arm linux/arm64" ./ssh-client
	$(dest)/gox -mod="readonly" -ldflags="-X main.Version=$(version) -w -s -H=windowsgui" -output="$(dest)/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="windows/amd64 linux/amd64 linux/arm linux/arm64"
	go run ./builder/archiver -osarch "windows/amd64 linux/amd64 linux/arm linux/arm64"
	rm $(dest)/gox*
	rm $(dest)/ssh-client_*
	rm $(dest)/my-ssh-manager_*

clean:
	rm -rf $(dest)/*.exe
	rm -rf $(dest)/my-ssh-manager*
	rm -rf $(dest)/ssh-client*
	rm -rf $(dest)/browser_data
	rm -rf dist
	rm -rf ssh-client/$(dest)/*
	rm -f ./coverage.html
	rm -f ./coverage.out
