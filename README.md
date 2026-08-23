# Browser Launcher

Windows 11でGoogle Chromeを指定した大きさと位置に起動するアプリです。

最初のMVPでは、プライマリディスプレイ上にChromeのウィンドウを1つ起動します。対応するブラウザ、OS、ウィンドウ数は小さく検証しながら拡張します。

本プロジェクトは、Go、React、DDD、クリーンアーキテクチャを学びながら、開発者本人が使用するツールを作ることを目的としています。ソースコードと開発過程は公開しますが、Windows向け実行ファイルやインストーラーは配布しません。

初期開発はPrivateリポジトリで行いました。秘密情報、個人情報、ライセンス、公開文書を監査した後、監査済みのファイルから新しいGit履歴を作成してPublic版を開始します。過去のPrivate版のコミットとPull RequestはPublic版へ移行しません。

## 設計方針

- ドメイン、アプリケーション、アダプター、インフラストラクチャの責務を分離する
- Goらしい単純な実装を優先し、小規模な製品に必要な範囲でDDDとクリーンアーキテクチャを適用する
- LinuxとWindowsのCIで自動検証し、ブラウザ操作などのWindows固有機能はWindows 11で実機検証する
- 実行ファイルを配布せず、開発者がWindows 11でローカルビルドする

## ドキュメント

- [ドキュメント一覧](./docs/README.md)
- [アーキテクチャ](./docs/develop/architecture/architecture.md)
- [製品要件](./docs/product/requirements.md)
- [検証記録](./docs/verification/)

## 開発環境

製品アプリケーションには、Wails v2.12.0、Go、React、TypeScriptを使用します。Windows 11での実行には、Microsoft Edge WebView2 Runtimeが必要です。

次の環境を準備します。

- Windows 11
- Go 1.23以上
- Node.js 22以上
- npm
- Microsoft Edge WebView2 Runtime

PowerShellで依存関係を取得し、WailsのTypeScriptバインディングを生成します。

```powershell
go mod download
npm.cmd ci
npm.cmd --prefix frontend ci
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/develop/generate-wails-bindings.ps1
```

テストとビルドを実行します。

```powershell
go test ./...
npm.cmd run lint:markdown
npm.cmd --prefix frontend run build
go build ./...
```

フロントエンドテストを実行します。

```powershell
npm.cmd --prefix frontend test -- --run
```

開発モードで起動します。

```powershell
go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0 dev
```

Wailsが生成する`frontend/wailsjs`、フロントエンドビルドが生成する`frontend/dist`、依存関係の`node_modules`はバージョン管理しません。
