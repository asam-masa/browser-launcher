# ADR-0005: Windows向けバイナリを配布せずローカルでビルドする

- Status: Accepted
- Date: 2026-08-14
- Related: SCRUM-38、SCRUM-40

## Context

Browser Launcherは、Go、React、DDD、クリーンアーキテクチャを学びながら開発するWindows向けデスクトップアプリケーションです。現在の利用者は開発者本人であり、ソースコードは学習過程を共有する目的で将来公開する予定です。一方、ビルド済み実行ファイルを一般利用者へ配布する予定はありません。

Wails v2で作成したWindowsアプリケーションは、Microsoft Edge WebView2 Runtimeを必要とします。第三者へバイナリを配布する場合は、実行ファイルの形式だけでなく、WebView2 Runtimeの導入、コード署名、バージョン、成果物名、ビルド環境、完全性確認、利用者サポートを決める必要があります。

本ADRでは、現在の目的に対してどこまで配布の仕組みを用意するかを決定します。リポジトリをpublicへ変更するための監査と、実際の公開操作はSCRUM-39で別に扱います。

## Decision Drivers

- 学習と製品機能の開発へ集中できること
- 開発者本人がWindows 11で実行できること
- 現在予定していない一般利用者向けの運用を増やさないこと
- WebView2 Runtime、署名証明書、リリース成果物を管理する責務が明確であること
- ビルド成果物を誤ってバージョン管理または公開しないこと
- 将来バイナリを配布するときに、配布方法を改めて選択できること

## Considered Options

### Option 1: バイナリを配布せず、開発者がローカルでビルドする

ソースコードと開発文書を管理し、実行ファイルは開発者がWindowsローカル環境で必要なときだけ作成します。

利点:

- インストーラー、署名証明書、配布用CIを管理する必要がない
- 一般利用者向けの導入手順とサポートを用意せず、学習と機能開発へ集中できる
- 配布方法を将来の利用者と要件に合わせて選び直せる

欠点:

- 第三者が試す場合は、開発環境を準備してソースコードからビルドする必要がある
- 配布物を使用したインストール、SmartScreen、別環境での起動は検証されない

リスクと軽減策:

- 将来の配布開始時に検討事項が集中する。配布が必要になった時点で、単体実行ファイル、インストーラー、WebView2、コード署名、CIを新しいADRで決定する
- ローカル成果物が公開される可能性がある。ビルド出力をバージョン管理から除外し、GitHub Releasesへ登録しない

検証方法:

- Windows 11のローカル環境で、文書化した開発手順からアプリケーションをビルドして起動する
- ビルド出力がGitの追跡対象にならないことを確認する

### Option 2: 単体実行ファイルを配布する

Wailsで生成した単体実行ファイルをGitHub Releasesなどで配布します。

利点:

- 利用者はインストーラーを使用せずにアプリケーションを試せる
- アプリケーションの削除に専用のアンインストール処理を必要としない

欠点:

- WebView2 Runtimeがない環境への導入方法を決める必要がある
- バージョン、成果物名、ハッシュ、保管場所、再現可能なビルド手順を管理する必要がある
- 未署名の実行ファイルには、Microsoft Defender SmartScreenの警告が表示される可能性がある

リスクと軽減策:

- 利用者が発行元と完全性を判断しにくい。配布開始前にコード署名とSHA-256の記録を検討する
- WebView2 Runtimeを導入できず起動できない可能性がある。Wailsの`webview2`オプションとMicrosoftのEvergreen Runtime配布方法を比較する

検証方法:

- クリーンなWindows環境で、WebView2 Runtimeの有無を含む起動確認を行う
- ダウンロード、SmartScreen、改ざん検知、更新時の置き換えを確認する

### Option 3: NSISインストーラーを配布する

Wailsが対応するNSISインストーラーを作成し、インストールとアンインストールを提供します。

利点:

- インストール先、ショートカット、アンインストールを一貫した手順で提供できる
- WebView2 Runtimeの導入をインストール手順へ組み込みやすい

欠点:

- インストーラー固有の設定、検証、保守が必要になる
- アプリケーション本体とインストーラーの両方について、署名と配布方法を検討する必要がある
- 現在の利用者と公開目的に対して運用負荷が大きい

リスクと軽減策:

- インストールやアンインストールで利用者環境へ不要な変更を残す可能性がある。対応するWindows環境で更新と削除を含む受入テストを行う
- 配布経路が増えて再現性が下がる可能性がある。インストーラー生成をCIへ集約し、使用ツールのバージョンを固定する

検証方法:

- クリーンなWindows環境で、インストール、起動、更新、アンインストールを確認する
- WebView2 Runtimeがない環境と既に存在する環境の両方を確認する

### ビルド環境の比較

#### Option A: Windowsローカル環境だけで検証する

利点:

- Chrome、Firefox、ディスプレイ、DPIを含む実機の振る舞いを確認できる
- GitHub Actionsのworkflowと権限を管理する必要がない

欠点:

- 検証コマンドの実行忘れを自動的に検出できない
- コミットまたはPRごとの検証結果がGitHubに残らない
- 開発環境の状態によって結果が変わる可能性がある

#### Option B: GitHub-hosted runnerによる検証用CIを併用する

利点:

- コミットまたはPRごとに、Unit Test、静的解析、フロントエンドビルド、Windows向けコンパイルを同じ手順で実行できる
- LinuxとWindowsのクリーンな環境で回帰を検出できる
- 検証結果をGitHubで追跡し、マージ判断に利用できる

欠点:

- workflow、Actionのバージョン、権限、runnerイメージの変更を管理する必要がある
- 実際のブラウザ、複数ディスプレイ、DPIを使用する実機検証は代替できない
- privateリポジトリではGitHub Actionsの無料利用枠を消費する

#### Option C: 配布用CI/CDを構築する

利点:

- バイナリ生成、署名、ハッシュ記録、GitHub Releasesへの登録を同じ手順で再現できる
- 開発者のローカル環境に依存しない配布物を作成できる

欠点:

- 署名資格情報、成果物、バージョン、リリース権限を管理する必要がある
- バイナリを配布しない現在の方針では、生成した成果物の利用先がない
- 検証用CIより権限と運用の範囲が広い

## Decision

Option 1を採用します。

配布形式はOption 1、ビルド環境はOption AとOption Bの併用を採用します。Option Cの配布用CI/CDは採用しません。

ビルド済みのWindows実行ファイルとインストーラーは配布しません。GitHub Releasesへ実行ファイルを登録せず、コード署名も実施しません。

開発者は、Windows 11のローカル環境で必要なときだけアプリケーションをビルドします。Wailsの出力ファイル名は`browser-launcher.exe`、ローカル出力先は`build/bin/`とします。ビルド成果物はGitで追跡しません。

検証用CIはSCRUM-40で導入します。GitHub-hosted runnerを使用して、Unit Test、race detector、静的解析、フロントエンドビルド、Windows向けコンパイルを自動実行します。CIで生成したバイナリは保存または配布しません。実際のブラウザ操作、ウィンドウ追跡、複数ディスプレイ、DPIの検証はWindowsローカル環境で継続します。

Microsoft Edge WebView2 Evergreen Runtimeは開発環境の前提条件とします。本プロジェクトからRuntime、Bootstrapper、Fixed Versionを配布しません。Runtimeが存在しない第三者の環境へ導入する方法は、現在の対象外とします。

配布用のバージョンと成果物名は定義しません。ソースコードとローカルビルドの状態はGitコミットで識別します。バイナリの配布を開始する場合は、SemVer、成果物名、SHA-256、再現可能なビルド環境をその時点で決定します。

ソースコードの公開はバイナリ配布と分離します。リポジトリをpublicへ変更する場合は、SCRUM-39で秘密情報、個人情報、ライセンス、公開文書、GitHub設定を監査します。

現時点では配布実装、配布物検証、配布運用の後続チケットを作成しません。バイナリを配布する要件が生じた場合は、本ADRを見直すADRと、配布実装・検証・運用準備のチケットを作成します。検証用CIの導入は、バイナリ配布とは分離してSCRUM-40で扱います。

## Consequences

- 学習と製品機能の開発に不要な配布運用を増やさずに済む
- インストーラー、コード署名証明書、配布用CI、リリース成果物を管理する必要がない
- 自動検証のために、GitHub Actionsのworkflow、権限、Actionのバージョンを管理する必要がある
- GitHub-hosted runnerの検証結果とWindows実機の検証結果を区別して記録する必要がある
- WebView2 Runtimeは開発者が自分の環境へ用意する必要がある
- 第三者がアプリケーションを試す場合は、ソースコードからビルドする必要がある
- ビルド済みバイナリを使ったSmartScreen、インストール、アンインストール、更新は検証対象にならない
- リポジトリをpublicにしても、実行ファイルの提供や利用者向けサポートを行うことを意味しない
- 第三者へバイナリを配布する要件が生じた場合は、本ADRの前提が変わるため新しいADRで見直す必要がある

## Validation

- [x] Wails v2.12.0を使用してWindows 11で開発モードを起動できる
- [x] Windows 11のローカル環境でフロントエンドとGoアプリケーションをビルドできる
- [x] Windows 11でビルドしたアプリケーションを起動し、MVPの操作を実行できる
- [x] `frontend/dist/`がバージョン管理から除外されている
- [x] `build/bin/`がバージョン管理から除外されている
- [x] SCRUM-40でGitHub-hosted runnerによる検証用CIを実行できる

SCRUM-40では、LinuxとWindowsの検証用CIが成功しました。初回実行ではGit管理対象外のWailsバインディングが存在せずFrontend Testに失敗したため、両OSでFrontend Test前にWails v2.12.0のバインディングを生成するよう修正しました。修正後は、LinuxでUnit Test、race detector、静的解析、フロントエンドビルドが成功し、WindowsでUnit Test、静的解析、フロントエンドビルド、Windows向けコンパイルが成功しました。

SCRUM-39のpublic化前監査と第三者向けのバイナリ配布方法は、現在のDecisionのValidation対象外です。将来配布する場合は、後続ADRとチケットで検証します。

## References

- [Wails v2.12.0: Windows](https://wails.io/docs/v2.12.0/guides/windows/): WailsのWindowsアプリケーションがWebView2 Runtimeを必要とすることと、Runtime不足時の選択肢
- [Wails v2.12.0: NSIS installer](https://wails.io/docs/v2.12.0/guides/windows-installer/): WailsがNSISによるWindowsインストーラー生成に対応すること
- [Microsoft Learn: EvergreenとFixed VersionのWebView2 Runtime](https://learn.microsoft.com/ja-jp/microsoft-edge/webview2/concepts/evergreen-vs-fixed-version): Evergreen Runtimeが多くのアプリケーションで推奨され、自動更新されること
- [Microsoft Learn: Windowsアプリ開発者向けSmartScreenの評価](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation): 未署名バイナリと署名済みバイナリに対するSmartScreenの評価
- [GitHub Docs: GitHub-hosted runners reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners): LinuxとWindowsの標準runner、およびpublic・privateリポジトリでの利用条件
- [ADR-0003: デスクトップアプリケーション基盤にWails v2を使用する](./0003-use-wails-v2-for-desktop-foundation.md)
