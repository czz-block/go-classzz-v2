# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: gczz android ios gczz-cross evm all test clean
.PHONY: gczz-linux gczz-linux-386 gczz-linux-amd64 gczz-linux-mips64 gczz-linux-mips64le
.PHONY: gczz-linux-arm gczz-linux-arm-5 gczz-linux-arm-6 gczz-linux-arm-7 gczz-linux-arm64
.PHONY: gczz-darwin gczz-darwin-386 gczz-darwin-amd64
.PHONY: gczz-windows gczz-windows-386 gczz-windows-amd64

GOBIN = ./build/bin
GO ?= latest
GORUN = env GO111MODULE=on go run

gczz:
	$(GORUN) build/ci.go install ./cmd/gczz
	@echo "Done building."
	@echo "Run \"$(GOBIN)/gczz\" to launch gczz."

all:
	$(GORUN) build/ci.go install

android:
	$(GORUN) build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/gczz.aar\" to use the library."
	@echo "Import \"$(GOBIN)/gczz-sources.jar\" to add javadocs"
	@echo "For more info see https://stackoverflow.com/questions/20994336/android-studio-how-to-attach-javadoc"

ios:
	$(GORUN) build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/Gczz.framework\" to use the library."

test: all
	$(GORUN) build/ci.go test

lint: ## Run linters.
	$(GORUN) build/ci.go lint

clean:
	env GO111MODULE=on go clean -cache
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go install golang.org/x/tools/cmd/stringer@latest
	env GOBIN= go install github.com/kevinburke/go-bindata/go-bindata@latest
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install github.com/golang/protobuf/protoc-gen-go@latest
	env GOBIN= go install ./cmd/abigen
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

# Cross Compilation Targets (xgo)

gczz-cross: gczz-linux gczz-darwin gczz-windows gczz-android gczz-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/gczz-*

gczz-linux: gczz-linux-386 gczz-linux-amd64 gczz-linux-arm gczz-linux-mips64 gczz-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-*

gczz-linux-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/gczz
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep 386

gczz-linux-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/gczz
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep amd64

gczz-linux-arm: gczz-linux-arm-5 gczz-linux-arm-6 gczz-linux-arm-7 gczz-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep arm

gczz-linux-arm-5:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/gczz
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep arm-5

gczz-linux-arm-6:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/gczz
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep arm-6

gczz-linux-arm-7:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/gczz
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep arm-7

gczz-linux-arm64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/gczz
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep arm64

gczz-linux-mips:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/gczz
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep mips

gczz-linux-mipsle:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/gczz
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep mipsle

gczz-linux-mips64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/gczz
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep mips64

gczz-linux-mips64le:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/gczz
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/gczz-linux-* | grep mips64le

gczz-darwin: gczz-darwin-386 gczz-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/gczz-darwin-*

gczz-darwin-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/gczz
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-darwin-* | grep 386

gczz-darwin-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/gczz
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-darwin-* | grep amd64

gczz-windows: gczz-windows-386 gczz-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/gczz-windows-*

gczz-windows-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/gczz
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-windows-* | grep 386

gczz-windows-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/gczz
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gczz-windows-* | grep amd64
