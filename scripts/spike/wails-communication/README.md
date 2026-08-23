# Wails連携方式の最小検証

## 目的

SCRUM-14で検討しているWails v2のメソッドバインディング、イベント、取消、タイムアウト、エラー、状態の再取得を、Chrome制御から分離して確認します。

本ディレクトリは検証専用です。製品実装のディレクトリ構成やUIを決定するものではありません。

## 検証環境

- Windows 11
- Go 1.23以上
- Node.js 22以上
- npm
- Microsoft Edge WebView2 Runtime
- Wails v2.12.0
- React 18.2.0
- Vite 6.4.3
- TypeScript 5.9.3

## 準備

PowerShellで本ディレクトリへ移動し、依存関係を取得します。

```powershell
go mod download
npm.cmd --prefix frontend ci
New-Item -ItemType Directory -Force -Path frontend/dist | Out-Null
Set-Content -Path frontend/dist/index.html -Value '<!doctype html><title>binding generation placeholder</title>'
go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0 generate module
npm.cmd --prefix frontend run build
```

Wails CLIはユーザー環境へ固定インストールせず、指定したバージョンを`go run`で実行します。

`frontend/dist`の仮ファイルは、初回のバインディング生成時にGoの埋め込み要件を満たすために作成します。後続のフロントエンドビルドが仮ファイルを置き換えます。

`frontend/wailsjs`はWailsが生成するため、バージョン管理しません。クリーン環境では、上記の順序でバインディングを生成してからフロントエンドをビルドしてください。

## 自動検証

操作レジストリーと状態通知の単体テストを実行します。

```powershell
go test ./...
```

Go 1.26.4のLinux環境では、書き込み可能なビルドキャッシュを指定して通常テストとrace detectorに成功しました。Go 1.26.5のWindows環境では、通常テストに成功しました。

準備後にフロントエンドを再検証する場合は、次のコマンドを実行します。

```powershell
npm.cmd --prefix frontend run build
```

## Windows実機検証

検証アプリを起動します。

```powershell
go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0 dev
```

画面から次を確認します。

- [x] 正常完了で`starting`、`running`、`completed`を受信できる
- [x] 実行中の操作を取り消すと`cancelling`と`cancelled`を受信できる
- [x] タイムアウトで`timed_out`を受信できる
- [x] 非同期失敗で`failed`とサニタイズされたエラーコードを受信できる
- [x] 開始前エラーがPromise rejectionとして表示される
- [x] 状態リスナーを解除するとイベント履歴が増えない
- [x] リスナー解除中に処理を完了しても、「最新状態を取得」で終端状態へ同期できる
- [x] 存在しない操作、取消処理中、完了済みの取消結果をGo側で区別できる
- [x] アプリケーションを終了した際に実行中の処理が終了する

### 検証結果

2026年7月26日にWindows 11で、すべての確認項目が成功しました。

取消の初回検証では、`cancelled`イベントの受信後に取消メソッドの`cancelling`応答が現在状態を上書きする競合を確認しました。Reactの状態更新をイベントと明示的な状態取得に限定し、応答による上書きを防止しました。

敵対的検証では、`CancelOperation`が`cancelling`を通知する前に処理goroutineが`cancelled`を通知する競合と、取消とタイマーが同時に成立した場合に`cancelling`で停止する経路を確認しました。処理goroutineへ取消状態の通知を集約し、タイマー側が選ばれた場合も取消済みなら`cancelled`へ収束するように修正しました。通知順序と収束はGoテストで確認し、Windows実機でも`starting`、`running`、`cancelling`、`cancelled`の順序を再確認しました。

開始処理では、Promiseの応答より先に受信したイベントをOperation IDごとに保持します。開始応答の`starting`が新しいイベント状態を上書きしないことを、Windows実機で確認しました。

Reactコンポーネントの再マウントは実施していません。イベントリスナーを解除した状態で操作を完了し、「最新状態を取得」で終端状態へ復旧できることは確認しました。

## 対象外

- Chromeの検出と起動
- WinEventによるウィンドウ追跡
- Win32 APIによる位置とサイズの補正
- 完成版UI
- インストーラーとコード署名

## 結果の記録

実行環境、各確認項目の結果、判明した制約を、`docs/research/go-react-desktop-foundation.md`へ記録します。
