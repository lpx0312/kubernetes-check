# k8s-patrol Makefile
# 用法：make help 查看所有目标
#
# 设计原则：
#   - 手动可跑（Linux 开发机直接 make build）
#   - CI 可调（GitHub Actions 调用相同 target，保证本地与流水线一致）
#   - 全平台交叉编译（amd64/arm64 × linux/windows/darwin）

BINARY_NAME  = k8s-patrol
CMD_DIR      = ./cmd/k8s-patrol
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE  ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS      = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)

# 默认平台（当前机器）
GOOS        ?= $(shell go env GOOS)
GOARCH      ?= $(shell go env GOARCH)

# 颜色输出
COLOR_RESET = \033[0m
COLOR_GREEN = \033[32m
COLOR_CYAN  = \033[36m

# 交叉编译目标矩阵：os-arch
TARGETS := linux-amd64 linux-arm64 windows-amd64 windows-arm64 darwin-amd64 darwin-arm64

.PHONY: help
help: ## 显示所有可用目标
	@printf "$(COLOR_CYAN)k8s-patrol Makefile$(COLOR_RESET)\n"
	@printf "用法: make [target]\n\n"
	@printf "$(COLOR_GREEN)开发:$COLOR_RESET\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_CYAN)%-18s$(COLOR_RESET) %s\n", $$1, $$2}'

.PHONY: build
build: ## 构建当前平台二进制（k8s-patrol）
	@printf "$(COLOR_GREEN)→ 构建 $(GOOS)/$(GOARCH)$(COLOR_RESET)\n"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_DIR)

.PHONY: build-all
build-all: $(addprefix cross-, $(TARGETS)) ## 交叉编译所有 6 个平台到 dist/

cross-%:
	$(eval OS := $(word 1,$(subst -, ,$*)))
	$(eval ARCH := $(word 2,$(subst -, ,$*)))
	@printf "$(COLOR_GREEN)→ 构建 $(OS)/$(ARCH)$(COLOR_RESET)\n"
	@mkdir -p dist
	$(eval EXT := $(if $(filter windows,$(OS)),.exe,))
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) \
		go build -trimpath -ldflags="$(LDFLAGS)" -o dist/$(BINARY_NAME)-$(OS)-$(ARCH)$(EXT) $(CMD_DIR)

.PHONY: dist-zip
dist-zip: build-all ## 打包 dist/ 下所有二进制为 .tar.gz / .zip（release 用）
	@printf "$(COLOR_GREEN)→ 打压 release 产物$(COLOR_RESET)\n"
	@cd dist && for f in k8s-patrol-linux-* k8s-patrol-darwin-*; do \
		[ -f "$$f" ] && tar -czf $$f.tar.gz $$f && rm $$f; \
	done
	@cd dist && for f in k8s-patrol-windows-*.exe; do \
		[ -f "$$f" ] && zip -q $${f%.exe}.zip $$f && rm $$f; \
	done
	@ls -lh dist/

.PHONY: test
test: ## 运行所有单元测试
	@printf "$(COLOR_GREEN)→ 单元测试$(COLOR_RESET)\n"
	go test -v ./internal/...

.PHONY: test-race
test-race: ## 运行单元测试（带竞态检测）
	go test -race ./internal/...

.PHONY: fmt
fmt: ## 格式化代码
	gofmt -s -w .

.PHONY: vet
vet: ## 静态检查
	go vet ./...

.PHONY: check
check: fmt vet test ## 格式化 + 静态检查 + 测试（提交前用）

.PHONY: tidy
tidy: ## 整理依赖
	go mod tidy

.PHONY: run
run: build ## 构建并运行（需 KUBECONFIG 环境变量）
	./$(BINARY_NAME) --help

.PHONY: install-completion
install-completion: build ## 安装 bash 自动补全到系统目录（需 root）
	@if [ "$$(id -u)" -ne 0 ]; then echo "需要 root 权限，请用 sudo make install-completion"; exit 1; fi
	@if ! command -v bash-completion >/dev/null 2>&1 && [ ! -f /usr/share/bash-completion/bash_completion ]; then \
		echo "请先安装 bash-completion: yum install -y bash-completion 或 apt install -y bash-completion"; exit 1; fi
	./$(BINARY_NAME) completion bash > /etc/bash_completion.d/k8s-patrol
	@printf "$(COLOR_GREEN)✓ 补全已安装，执行 source /etc/profile.d/bash_completion.sh 或重新登录生效$(COLOR_RESET)\n"

.PHONY: clean
clean: ## 清理构建产物
	rm -rf dist $(BINARY_NAME) k8s-patrol.exe

.PHONY: docker-build
docker-build: ## 在 Docker 中构建 Linux 二进制（无需本地 Go 环境）
	docker run --rm -v $(PWD):/app -w /app golang:1.22-alpine \
		sh -c 'go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_DIR)'
