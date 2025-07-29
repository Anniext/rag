# RAG 系统 Makefile

.PHONY: build test clean lint fmt vet deps run help

# 变量定义
APP_NAME := rag
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT_SHA := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GO_VERSION := $(shell go version | awk '{print $$3}')

# 构建标志
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.BuildTime=$(BUILD_TIME) \
           -X main.CommitSHA=$(COMMIT_SHA) \
           -X main.GoVersion=$(GO_VERSION)

# 默认目标
all: build

# 显示帮助信息
help:
	@echo "RAG 系统构建工具"
	@echo ""
	@echo "可用命令:"
	@echo "  build     构建应用程序"
	@echo "  test      运行测试"
	@echo "  clean     清理构建产物"
	@echo "  lint      代码检查"
	@echo "  fmt       格式化代码"
	@echo "  vet       静态分析"
	@echo "  deps      安装依赖"
	@echo "  run       运行应用程序"
	@echo "  help      显示此帮助信息"

# 安装依赖
deps:
	@echo "📦 安装依赖..."
	go mod download
	go mod tidy
	go mod verify

# 格式化代码
fmt:
	@echo "🎨 格式化代码..."
	go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	fi

# 静态分析
vet:
	@echo "🔬 静态分析..."
	go vet ./...

# 代码检查
lint: vet
	@echo "🔍 代码检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "警告: golangci-lint 未安装，跳过 lint 检查"; \
	fi

# 运行测试
test:
	@echo "🧪 运行测试..."
	go test -v -race -coverprofile=coverage.out ./...

# 测试覆盖率
test-coverage: test
	@echo "📊 生成测试覆盖率报告..."
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out

# 基准测试
benchmark:
	@echo "⚡ 运行基准测试..."
	go test -bench=. -benchmem ./...

# 构建应用
build: deps fmt vet
	@echo "🔨 构建应用..."
	@echo "版本: $(VERSION)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "提交: $(COMMIT_SHA)"
	CGO_ENABLED=0 go build \
		-ldflags "$(LDFLAGS)" \
		-o bin/$(APP_NAME) \
		./main.go

# 构建多平台版本
build-all: deps fmt vet
	@echo "🔨 构建多平台版本..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-linux-amd64 ./main.go
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-linux-arm64 ./main.go
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-darwin-amd64 ./main.go
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-darwin-arm64 ./main.go
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME)-windows-amd64.exe ./main.go

# 运行应用
run: build
	@echo "🚀 启动应用..."
	./bin/$(APP_NAME)

# 开发模式运行
dev:
	@echo "🚀 开发模式启动..."
	go run -ldflags "$(LDFLAGS)" ./main.go

# 清理构建产物
clean:
	@echo "🧹 清理构建产物..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache
	go clean -modcache

# 安全扫描
security:
	@echo "🔒 安全扫描..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "警告: gosec 未安装，跳过安全扫描"; \
	fi

# 生成文档
docs:
	@echo "📚 生成文档..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "启动文档服务器: http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "警告: godoc 未安装"; \
	fi

# 检查代码质量
quality: fmt vet lint test security
	@echo "✅ 代码质量检查完成"

# 完整构建流程
ci: quality build
	@echo "✅ CI 流程完成"

# 安装开发工具
install-tools:
	@echo "🛠️ 安装开发工具..."
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install golang.org/x/tools/cmd/godoc@latest

# 初始化项目
init: install-tools deps
	@echo "🎉 项目初始化完成"

# Docker 构建
docker-build:
	@echo "🐳 构建 Docker 镜像..."
	docker build -t $(APP_NAME):$(VERSION) .
	docker build -t $(APP_NAME):latest .

# Docker 运行
docker-run:
	@echo "🐳 运行 Docker 容器..."
	docker run --rm -p 8080:8080 -p 8081:8081 $(APP_NAME):latest

# 显示版本信息
version:
	@echo "应用名称: $(APP_NAME)"
	@echo "版本: $(VERSION)"
	@echo "构建时间: $(BUILD_TIME)"
	@echo "提交: $(COMMIT_SHA)"
	@echo "Go 版本: $(GO_VERSION)"