dest = bin

ifndef version
#	version = 0.0.1
	version = dev
endif

build:
	go build -ldflags "-w -s" -trimpath -o $(dest)/

dist:
	go get -d github.com/mitchellh/gox
	go build -mod=readonly -o $(dest)/ github.com/mitchellh/gox
	go mod tidy
	go env -w GOFLAGS=-trimpath
	$(dest)/gox -mod="readonly" -ldflags="-X main.Version=$(version) -w -s" -output="$(dest)/{{.Dir}}_{{.OS}}_{{.Arch}}" -osarch="windows/amd64 linux/amd64 linux/arm linux/arm64 darwin/amd64 darwin/arm64"
	rm $(dest)/gox*