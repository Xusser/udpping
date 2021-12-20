export PATH := $(GOPATH)/bin:$(PATH)
export GO111MODULE=on
LDFLAGS := -s -w
OS := windows linux darwin openbsd
ARCH := amd64 386 arm arm64
OSARCH := !darwin/arm !darwin/386
OUTPUT := dist/udpping_{{.OS}}_{{.Arch}}

all: clean fmt build

build: go upx

fmt:
	go fmt ./...

go:
	gox -ldflags "$(LDFLAGS)" -os="${OS}" -arch="${ARCH}" -osarch="$(OSARCH)" -output="$(OUTPUT)"

upx:
	upx --lzma dist/udpping_linux*
	upx --lzma dist/udpping_windows*
	upx --lzma dist/udpping_darwin*
	
clean:
	rm -f ./dist/*