# Go道場 - 開発支援Makefile
.PHONY: help test test-all test-day lint fmt vet clean deps setup

# デフォルトターゲット
help: ## このヘルプメッセージを表示
	@echo "Go道場 - 開発支援コマンド"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# テスト関連
test: ## 現在のディレクトリのテストを実行
	go test -v ./...

test-all: ## 全ての日の課題のテストを実行
	@echo "全課題のテストを実行中..."
	@for dir in day*/; do \
		if [ -d "$$dir" ] && [ -f "$$dir/main_test.go" ]; then \
			echo "Testing $$dir..."; \
			(cd "$$dir" && go test -v) || echo "$$dir: FAILED"; \
		fi \
	done

test-day: ## 特定の日の課題をテスト (DAY=01 形式で指定)
	@if [ -z "$(DAY)" ]; then \
		echo "使用方法: make test-day DAY=01"; \
		exit 1; \
	fi
	@dir=$$(find . -name "day$(DAY)-*" -type d); \
	if [ -n "$$dir" ]; then \
		echo "Testing $$dir..."; \
		cd "$$dir" && go test -v; \
	else \
		echo "Day $(DAY) が見つかりません"; \
		exit 1; \
	fi

bench-day: ## 特定の日の課題でベンチマークを実行
	@if [ -z "$(DAY)" ]; then \
		echo "使用方法: make bench-day DAY=01"; \
		exit 1; \
	fi
	@dir=$$(find . -name "day$(DAY)-*" -type d); \
	if [ -n "$$dir" ]; then \
		echo "Benchmarking $$dir..."; \
		cd "$$dir" && go test -bench=. -benchmem; \
	else \
		echo "Day $(DAY) が見つかりません"; \
		exit 1; \
	fi

# コード品質
lint: ## golangci-lintでコードをチェック
	golangci-lint run ./...

fmt: ## go fmtでコードをフォーマット
	go fmt ./...

vet: ## go vetでコードを静的解析
	go vet ./...

# セットアップ
deps: ## 依存関係をインストール
	go mod download
	go mod tidy

setup: ## 開発環境をセットアップ
	@echo "Go道場の開発環境をセットアップ中..."
	go mod init go-dojo || true
	go get github.com/ory/dockertest/v3
	go get github.com/lib/pq
	go get github.com/go-redis/redis/v8
	go mod tidy
	@echo "セットアップ完了！"

# ユーティリティ
clean: ## 一時ファイルを削除
	go clean ./...
	rm -f coverage.out

coverage: ## テストカバレッジを測定
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "カバレッジレポート: coverage.html"

progress: ## 進捗を確認
	@echo "進捗状況:"
	@if [ -f progress.csv ]; then \
		awk -F',' 'NR>1 && $$3=="Completed" {completed++} NR>1 {total++} END {print "完了: " completed "/" total " (" int(completed/total*100) "%)"} $$3=="In Progress" {print "進行中: " $$2}' progress.csv; \
	else \
		echo "progress.csvが見つかりません"; \
	fi

# Docker関連（統合テスト用）
docker-up: ## テスト用のDockerコンテナを起動
	docker-compose up -d postgres redis

docker-down: ## テスト用のDockerコンテナを停止
	docker-compose down

# 特定の日の課題の作成（開発者用）
create-day: ## 新しい日の課題を作成 (DAY=XX TOPIC=topic-name 形式)
	@if [ -z "$(DAY)" ] || [ -z "$(TOPIC)" ]; then \
		echo "使用方法: make create-day DAY=XX TOPIC=topic-name"; \
		exit 1; \
	fi
	@mkdir -p day$(DAY)-$(TOPIC)
	@echo "Day $(DAY) - $(TOPIC) のディレクトリを作成しました"