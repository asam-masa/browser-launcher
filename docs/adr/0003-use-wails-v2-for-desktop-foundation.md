# ADR-0003: デスクトップアプリケーション基盤にWails v2を使用する

- Status: Accepted
- Date: 2026-07-26
- Related: SCRUM-14

## Context

Browser Launcherは、Goでアプリケーション処理を実装し、ReactでデスクトップUIを提供します。ReactはWebView内で動作するため、GoのApplication層を呼び出し、非同期処理の状態を受け取る境界が必要です。

本プロジェクトの「通信」は外部ネットワーク通信ではなく、デスクトップアプリケーション内でReactとGoを接続する境界を指します。Chromeの起動、プロファイル選択、ウィンドウ追跡、配置には時間がかかるため、同期的な要求と非同期の状態通知を分ける必要があります。

SCRUM-14では、Wails v2のメソッドバインディング、TypeScript型生成、イベント、取消、タイムアウト、エラー通知、状態再取得を最小構成で検証しました。

## Decision Drivers

- GoとReactをデスクトップアプリケーション内で接続できること
- ローカルHTTPサーバーや待受ポートを管理しないこと
- DomainとApplicationをUIフレームワークから分離できること
- Reactから型のある要求を送信できること
- Goから非同期処理の状態を通知できること
- イベントの欠落やReactの再マウント後に状態を復元できること
- 取消、タイムアウト、失敗を異なる結果として扱えること
- Windows 11でビルド、起動、終了を検証できること
- 小規模な学習プロジェクトに見合う実装量であること

## Considered Options

### Option 1: Wails v2を使用する

Wailsのメソッドバインディングとイベントを使用して、ReactとGoを接続します。

利点:

- GoのメソッドとDTOからTypeScriptのバインディングを生成できる
- WebViewとGoの連携機構を独自実装せずに済む
- ローカルHTTPサーバーと待受ポートが不要になる
- Windows向けデスクトップアプリケーションのライフサイクルを管理できる

欠点:

- Wails固有の生成処理とランタイムへ依存する
- WindowsではMicrosoft Edge WebView2 Runtimeが必要になる
- Wails境界をApplicationやDomainへ漏らさない設計が必要になる

### Option 2: ローカルHTTP APIとWebSocketを使用する

Goでローカルサーバーを起動し、ReactからHTTPとWebSocketで接続します。

利点:

- Web標準の通信方式と開発ツールを使用できる
- UIとバックエンドを別プロセスとして分離しやすい

欠点:

- ポートの確保、起動待機、終了処理、接続障害を扱う必要がある
- ローカル通信の認証、オリジン、公開範囲を設計する必要がある
- 単一のデスクトップアプリケーションには運用要素が多い

### Option 3: WebView2を直接統合する

WindowsのWebView2 APIを使用し、JavaScriptとGoの連携を実装します。

利点:

- Windows固有のWebView機能を細かく制御できる
- Wails固有の抽象化へ依存しない

欠点:

- WebViewの生成、ライフサイクル、メッセージ変換を実装する必要がある
- Goとの連携コードとWindows固有コードが増える
- 本プロジェクトの目的に対して実装量が大きい

### Option 4: Wails v3を使用する

Wails v3の新しいAPIとデスクトップ機能を使用します。

利点:

- 新しい設計と機能を利用できる
- 将来のWails開発方針に近い

欠点:

- 2026年7月時点ではAlphaであり、APIと移行手順が安定していない
- 学習対象へフレームワーク更新への追従が加わる

## Decision

Option 1を採用し、デスクトップアプリケーション基盤にWails v2.12.0を使用します。フロントエンドにはReactとTypeScriptを使用します。

ReactからGoへの要求にはWailsのメソッドバインディングを使用します。Wailsへ公開するControllerはInterface Adapterに配置し、DTOの検証とApplication向けの型変換を担当します。Domain型、Application Service、Infrastructure実装はWailsへ直接バインドしません。

短時間で受け付けられる要求は、メソッドの戻り値またはPromise rejectionで結果を返します。時間のかかる処理は、開始時にOperation IDを返して非同期で実行します。

GoからReactへの状態通知にはWailsイベントを使用します。ただし、イベントは一時的な通知であり、状態のSingle Source of Truth（SSOT）にはしません。Go側の`OperationRegistry`が、Operation ID、現在状態、終了結果、取消関数を保持します。

`OperationRegistry`は、実行中の操作とその終端状態を最新の1件だけ保持します。終端状態は、Reactがイベント欠落後に再取得できるよう、次の操作が始まるまで保持します。新しい操作を開始した時点で直前の終端状態を置き換え、過去の操作履歴は保持しません。Operation IDはアプリケーションプロセス内で一意とし、アプリケーション終了後には状態を復元しません。

Reactは処理開始前にイベントリスナーを登録します。イベントを受信できなかった場合やReactを再マウントした場合は、状態取得メソッドを呼び出して`OperationRegistry`の最新状態と同期します。ReactはOperation IDごとの最新イベントを保持し、遅れて返ったメソッド応答で新しい状態を上書きしません。

製品UIでは`launcher:state-changed`イベントを使用し、`starting`、`running`、`cancelling`、`completed`、`cancelled`、`timed_out`、`failed`を通知します。Reactはイベントリスナーの登録後に現在の操作を取得します。状態取得中にイベントを受信した場合は、先に受信した新しいイベントを遅れて返る取得結果で上書きしません。

取消はイベントではなく、Operation IDを指定するメソッドとして扱います。Go側は`cancelling`を経て`cancelled`へ遷移します。処理がすぐに終了した場合は、古い`cancelling`を終端状態の後に通知せず、`cancelled`だけを通知することがあります。取消とタイマーが同時に成立した場合も、取消を受け付けた操作は`cancelled`へ収束させます。

Chrome起動処理はApplication Serviceが非同期で開始し、Workflowの完了を待たずにOperation IDを返します。Application Serviceは処理全体に60秒の期限を設定し、Workflowの結果を`completed`、`cancelled`、`timed_out`、`failed`のいずれかへ変換します。終端状態とエラーコードは`OperationRegistry`で一度に確定し、受け付け済みの取消をほかの完了結果で上書きしません。Workflowで予期しない`panic`が発生した場合は、`unexpected_failure`を伴う`failed`へ変換します。

開始受付前の入力不備や初期化失敗は、開始メソッドのPromise rejectionとして返します。Operation IDを返した後の成功、取消、タイムアウト、想定内の失敗、予期しない失敗は、状態イベントと状態取得結果で通知します。予期しない失敗では、内部パス、スタックトレース、OSエラーの詳細をReactへ渡しません。

ローカルHTTPサーバー、WebSocket、待受ポートは使用しません。Wails v3への移行はMVPの対象外とします。

Wailsが生成する`frontend/wailsjs`と、フロントエンドビルドが生成する`frontend/dist`はバージョン管理しません。Wails CLIのバージョンとフロントエンド依存関係を固定し、クリーン環境で生成します。依存関係を再現するロックファイルはバージョン管理します。

## Consequences

- Wails固有コードをInterface Adapterとエントリーポイントへ限定する必要がある
- ApplicationとDomainはWails、React、WebView2、Win32 APIの型へ依存しない
- ReactとGoの契約変更時にTypeScriptバインディングを再生成する必要がある
- 非同期処理ごとにOperation IDと状態遷移を管理する必要がある
- イベントだけでなく、最新状態を取得する経路を維持する必要がある
- 新しい操作を開始すると直前の終端状態を取得できなくなるため、Reactは保持しているOperation IDの状態を次の操作前に同期する必要がある
- 期限到達後は状態を`timed_out`へ確定するが、Contextへ協調しない内部処理を強制停止できないため、時間のかかる境界ではContextの確認または個別の時間制限が必要になる
- Windows環境ではWebView2 Runtimeの存在を確認し、不足時の案内方法を決定する必要がある
- Wails v3を採用する場合は、別のADRで移行理由と影響を判断する必要がある

## Validation

- [x] Wails v2.12.0のReact＋TypeScript構成をWindows 11で起動できる
- [x] ReactからDTOを渡してGoメソッドを呼び出せる
- [x] GoのDTOからTypeScriptバインディングを生成できる
- [x] GoからReactへOperation IDを含む状態イベントを通知できる
- [x] 取消とタイムアウトを異なる状態として受け取れる
- [x] イベント欠落後に状態取得メソッドで終端状態へ同期できる
- [x] 取消とタイマーの競合時に`cancelled`へ収束できる
- [x] 遅れて返る開始応答で新しいイベント状態を上書きしない
- [x] 開始前と開始後の失敗を異なる経路で通知できる
- [x] 実行中の処理を残した状態でアプリケーションを終了できる
- [x] Goの通常テストとrace detectorが成功する
- [x] TypeScriptコンパイルとViteビルドが成功する
- [x] npm監査で既知脆弱性が0件である
- [x] Reactを実際に再マウントした後に状態を復元できる

2026-08-30に、製品Reactテストの「再マウント時にApplicationの最新状態を復元する」で
確認しました。このテストは、コンポーネントのアンマウント時にイベントの購読を解除し、
再マウント時にイベントを再登録して、`GetCurrentLaunchState`から同じOperation IDの
実行中状態を復元できることを確認しています。

この自動テストは、モックを使用したReactコンポーネントテストです。Windows実機での
Wailsイベントとメソッドバインディングを使用した再マウント確認を置き換えるものではありません。
