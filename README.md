# Go道場プロフェッショナル編：全60日カリキュラム

プロダクションレベルのGoアプリケーション開発に必要なスキルを身につける実践的なカリキュラムです。

## プロジェクト概要

このリポジトリは、Goエンジニアがプロフェッショナルレベルに到達するための60日間の実践課題を提供します。
各日の課題は実際のプロダクション環境で遭遇する問題を模擬しており、テスト駆動で学習を進められます。

## ディレクトリ構成

```
go-dojo/
├── day01-context-cancellation/    # Day 01: Contextによるキャンセル伝播
├── day02-context-timeout/         # Day 02: Contextによるタイムアウト/デッドライン
├── day03-mutex-vs-rwmutex/        # Day 03: sync.Mutex vs RWMutex
├── ...                            # Day 04-60の課題
├── lib/                           # 複数の課題で共通して使う便利コード
│   └── tester/                    # dockertestを使ったDBセットアップヘルパー
├── tools/                         # 開発支援ツール
│   ├── dojo-cli/                  # 学習管理ツール
│   └── Makefile                   # 頻繁に使うコマンドのショートカット
├── progress.csv                   # 学習の進捗記録
└── README.md                      # このファイル
```

## カリキュラム概要

### Days 1-15: 高度な並行処理とデザインパターン
- Context、Mutex、Worker Pool、Pipeline、Rate Limiterなど

### Days 16-30: プロダクションレベルのWeb API
- HTTP Server設定、ミドルウェア、認証、テスト戦略など

### Days 31-45: データベースとキャッシュ戦略
- トランザクション、N+1問題、コネクションプール、Redisキャッシュなど

### Days 46-60: 分散システムと可観測性
- gRPC、メッセージキュー、Prometheus、OpenTelemetryなど

## 使い方

1. 各日のディレクトリに移動
2. README.mdで課題内容を確認
3. main.goに実装を記述
4. `go test`でテストを実行
5. 全テストが通過したら次の日へ

```bash
cd day01-context-cancellation
cat README.md          # 課題説明を読む
go test -v             # テスト実行
go test -bench=.       # ベンチマーク実行（該当する場合）
```

## 必要な前提知識

- Go言語の基本文法
- Goroutineとチャネルの基礎
- HTTP サーバーの基本
- SQL/データベースの基礎

## 推奨開発環境

- Go 1.21以上
- Docker & Docker Compose
- PostgreSQL（一部課題で使用）
- Redis（一部課題で使用）

## 進捗管理

`progress.csv`ファイルで学習進捗を記録できます。各日の完了状況、かかった時間、学んだことなどを記録しましょう。