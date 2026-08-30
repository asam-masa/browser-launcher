# Public版リポジトリ移行

## 状態

2026年8月23日に移行を完了しました。Private版は既存履歴を維持してPrivateのまま保管し、監査済みファイルから作成したPublic版で今後の開発を継続します。

## 目的

現在のPrivateリポジトリとGit履歴を変更せずに保管し、監査済みのファイルからPublic版の新しいGit履歴を作成します。学習成果と今後の開発過程をポートフォリオとして公開し、Publicリポジトリで検証用CIとGitHubのセキュリティ機能を利用します。

Windows向け実行ファイル、インストーラー、署名済みバイナリは配布しません。

## 設計判断

| 項目 | 決定 | 理由 |
| --- | --- | --- |
| Public版 | `asam-masa/browser-launcher` | 製品名と一致する簡潔な名前を、今後の正式な参照先にするため |
| Private版 | `asam-masa/browser-launcher-private` | 既存履歴のSSOTであることと、Public版との違いを明確にするため |
| Goモジュール | `github.com/asam-masa/browser-launcher` | Public版のURLと一致しており、ソースとimportの変更が不要なため |
| Git履歴 | Public版は1つの初期コミットから開始 | Private版の履歴を変更せず、過去の個人メールを公開対象から除外するため |
| ローカル配置 | Private版とPublic版を別ディレクトリで管理 | リポジトリと履歴の取り違えを防ぐため |

Private版の改名はGit履歴を書き換えません。Public版には、移行直前に確定した管理対象ファイルだけを複製します。

## 公開対象

基準コミットで`git ls-files`が返す管理対象ファイルを候補とし、次を確認してからPublic版へ複製します。

- 製品コード、テスト、開発スクリプト
- `README.md`、`LICENSE`、`SECURITY.md`
- 設計、開発、調査、検証の文書
- GitHub Actions、Pull Requestテンプレート
- AIエージェントの規約とSkill
- Goとnpmの依存関係定義およびロックファイル

## 除外対象

次はPublic版へ移行しません。

- `.git/`とPrivate版のGit履歴
- リポジトリ単位および利用者単位のGit設定
- `node_modules/`、`frontend/node_modules/`
- `frontend/wailsjs/`、`frontend/dist/`、`build/bin/`
- `.env`と環境変数の値
- JiraのアカウントメールとAPIトークン
- Gitleaksの検出値
- その他の管理対象外ファイル

## 移行手順と中止条件

1. Private版の最終差分、公開対象、除外対象を確認する。
2. Private版を`browser-launcher-private`へ改名し、可視性がPrivateのままであることを確認する。
3. リポジトリ外の一時領域へ、監査対象の管理対象ファイルだけを複製する。
4. 移行対象をバージョン固定したGitleaksで検査する。
5. Markdown lint、Frontend Test、Frontend Build、Go Test、race detector、Go Vet、Go Buildを実行する。
6. ID付き`noreply`メールが設定されていることを、値を表示せずに確認する。
7. Public版の新しいGit履歴と初期コミットを作成する。
8. `browser-launcher`をPrivateリポジトリとして作成し、初期コミットをpushする。
9. LinuxとWindowsのCI、およびGitHub上の公開表示候補を確認する。
10. Public化直前のファイルとGit履歴をGitleaksで再検査する。
11. 対象と影響を提示し、Public化について個別承認を得る。
12. 可視性をPublicへ変更し、セキュリティ設定と公開表示を確認する。

次のいずれかに該当する場合はPublic化せず、原因を確認します。

- 秘密情報または公開できない個人情報を検出した
- Private版の履歴または可視性に想定外の変更がある
- 初期コミットの作者・コミッターメールを安全に確認できない
- LinuxまたはWindowsのCIが失敗している
- Goモジュールパス、文書、GitHub設定に不整合がある
- Public化の個別承認を得ていない

## 検証結果

### 移行前検証

2026年8月23日に、作業ツリーからGit管理対象および追加予定のファイル177件を一時領域へ複製し、Public版候補として検証しました。

| 確認項目 | 結果 |
| --- | --- |
| Gitleaks | v8.29.1のLinux x64配布物が公式チェックサムと一致し、Public版候補の検出は0件 |
| Markdown | Markdown lintとローカルリンク検査が成功 |
| Frontend | 10件のテストとFrontend Buildが成功 |
| Go | Unit Test、Go Vet、Go Buildが成功 |
| race detector | WSL環境にCコンパイラーがないため未実施。Linux CIで確認する |
| Private版 | `asam-masa/browser-launcher-private`へ改名後もPrivate、未アーカイブ、既定ブランチ`main`、基準コミットが維持されている |
| Public版 | `asam-masa/browser-launcher`をPrivateで作成し、初期コミット`22c1437`から新しい履歴を開始 |
| GitHub Actions | 初期コミットと公開直前の`9664225`に対するLinuxとWindowsの検証が成功 |

検出0件は秘密情報が存在しないことを保証しません。Public版の初期コミット作成後にもGit履歴を再検査し、検出0件を確認しました。

### Public化前のGitHub設定

2026年8月23日に、Public版候補がPrivateの状態でGitHub APIから確認しました。

| 確認項目 | 結果 |
| --- | --- |
| Actions | 有効。許可するActionは`all`、SHA固定の強制は無効 |
| `GITHUB_TOKEN` | 既定権限はread、Pull Requestの承認は不可 |
| Secret scanningとPush protection | `security_and_analysis`を取得できず、未確認 |
| Dependabot alerts | APIがHTTP 404で終了し、未確認 |
| Dependabot security updates | Automated security fixes APIはHTTP 200。実際の有効状態は未確認 |
| Private vulnerability reporting | APIがHTTP 404で終了し、未確認 |
| Ruleset | APIがHTTP 403で終了し、未確認 |

workflowで使用するActionはコミットSHAへ固定済みです。Public化後に利用可能な設定を有効化し、APIとGitHub上の表示から再確認します。

### 移行後検証

2026年8月23日に、Public化とセキュリティ設定の適用後に確認しました。

| 確認項目 | 結果 |
| --- | --- |
| Public版 | Public、未アーカイブ、既定ブランチは`main` |
| 公開表示 | 未認証アクセスでリポジトリ、`README.md`、`LICENSE`、`SECURITY.md`がHTTP 200 |
| Git履歴 | 公開直前の2コミットで、すべての作者・コミッターメールがID付き`noreply`形式 |
| Gitleaks | 公開直前の全Git履歴で検出0件 |
| Private版 | Private、未アーカイブ、既定ブランチ`main`と基準コミットを維持 |
| Secret scanning | 有効 |
| Push protection | 有効 |
| Dependabot alerts | 有効 |
| Dependabot security updates | 有効 |
| Private vulnerability reporting | 有効。「Report a vulnerability」の表示を確認 |
| Actions | 既定の`GITHUB_TOKEN`権限はread、Pull Requestの承認は不可 |
| ForkからのPull Request | すべての外部コントリビューターによるworkflow実行に承認を要求 |
| Ruleset | `Protect main`をActiveにし、既定ブランチへ適用 |

`Protect main`では、Pull Request、会話の解決、最新の`main`への追従、`Linux verification`、`Windows verification`、linear history、squash mergeを必須にしています。ブランチ削除とForce pushは禁止し、bypass権限は設定していません。承認レビュー数は、個人開発で自己承認を必須にしないため0件です。

2026年8月30日に、GitHubのPR一覧とCheck resultsでPublic化後のフォローアップPRを確認しました。

| 対象 | 結果 |
| --- | --- |
| SCRUM-51 | PR #3を2026年8月26日に`main`へマージ済み。`Linux verification`と`Windows verification`は成功 |
| SCRUM-52 | PR #5を2026年8月30日に`main`へマージ済み。`Linux verification`と`Windows verification`は成功 |
| SCRUM-53 | PR #4を2026年8月29日に`main`へマージ済み。`Linux verification`と`Windows verification`は成功 |

## 既知脆弱性への対応

Public化後にDependabot alertsを有効にした結果、ルートと`wails-communication` Spikeの`golang.org/x/crypto`と`golang.org/x/net`に38件のOpen alertを確認しました。Dependabot PR #1はSpikeの`golang.org/x/crypto`だけを更新するため、ルートとSpikeを同じ変更で整合させるSCRUM-51へ置き換えます。

2026年8月23日の敵対的検証では、Go 1.26.4を使用した`govulncheck v1.7.0`により、ルートとSpikeの両方でGo標準ライブラリの`GO-2026-5972`が到達可能と判定されました。依存パッケージの既知脆弱性は、脆弱な関数を呼び出しているとは判定されませんでした。

2026年8月26日に、SCRUM-51の作業ブランチで次を更新しました。

| 対象 | 更新前 | 更新後 |
| --- | --- | --- |
| Go | 1.23.0以上 | 1.26.6以上 |
| `golang.org/x/crypto` | v0.33.0 | v0.52.0 |
| `golang.org/x/net` | v0.35.0 | v0.55.0 |
| `golang.org/x/sys` | v0.30.0 | v0.45.0 |
| `golang.org/x/text` | v0.22.0 | v0.37.0 |

ルートとSpikeでは、Go 1.26.6によるUnit Test、Go Vet、Go Build、`govulncheck v1.7.0`が成功しました。Spikeは、管理対象外のWailsバインディングとFrontend成果物を生成してから検証しました。`govulncheck`で到達可能な既知脆弱性は検出されませんでした。

2026年8月30日に、SCRUM-52の作業ブランチでEchoをv4.13.3からv4.15.4へ更新しました。v4.15.3は`GHSA-vfp3-v2gw-7wfq`の修正版ですが、v4.15.4では同じ脆弱性への後続修正として、静的ファイルのパスを既定でアンエスケープしない動作へ変更されています。このため、Dependabot PR #2が提案したv4.15.3ではなくv4.15.4を採用しました。

ルートとSpikeでは、Echoが必要とする次の推移依存も同じバージョンへ更新しました。

| 対象 | 更新前 | 更新後 |
| --- | --- | --- |
| `github.com/labstack/echo/v4` | v4.13.3 | v4.15.4 |
| `github.com/labstack/gommon` | v0.4.2 | v0.5.0 |
| `github.com/mattn/go-colorable` | v0.1.13 | v0.1.15 |
| `github.com/mattn/go-isatty` | v0.0.20 | v0.0.22 |
| `golang.org/x/crypto` | v0.52.0 | v0.53.0 |
| `golang.org/x/net` | v0.55.0 | v0.56.0 |
| `golang.org/x/sys` | v0.45.0 | v0.46.0 |
| `golang.org/x/text` | v0.37.0 | v0.38.0 |

Linuxでは、両モジュールの`go mod tidy -diff`、Unit Test、Go Vet、Go Build、`govulncheck v1.7.0`が成功しました。`govulncheck`では既知脆弱性を検出しませんでした。この時点では、Windows検証、Pull RequestのCI、マージ後のDependabot alert解消、Dependabot PR #2のクローズは未実施でした。

2026年8月30日に、GitHubのCheck resultsでPR #3とPR #5の`Linux verification`および`Windows verification`が成功していることを確認しました。両PRは`main`へマージ済みです。GitHubのDependabot alertsで確認したOpen alertは0件であり、置き換え元のDependabot PR #1とPR #2は未マージでクローズ済みです。

2026年8月30日に、SCRUM-55でSpike frontendの`nanoid`をv3.3.16からv3.3.18へ更新しました。クリーン環境で`npm ci`、`npm audit`、Frontend Buildが成功し、既知脆弱性が0件であることを確認しました。PR #7の`Linux verification`と`Windows verification`は成功し、`main`へマージ済みです。

SCRUM-56では、Linux・Windows CIでSpike frontendの依存関係取得、Wailsバインディング生成、Frontend Buildを実行します。Linux CIでは`npm audit`も実行し、High severity以上の既知脆弱性を検出した場合は失敗させます。

## 残存リスクと未確認事項

- Gitleaksの検出0件は、未知の形式や分割された秘密情報が存在しないことを保証しない
- 実在する外部コントリビューターのForkからPull Requestを作成する検証は未実施
- Linux CIでGoキャッシュの復元警告が発生したが、race detectorを含む全ステップとジョブは成功
- Rulesetで必須チェック名を固定しているため、workflowのジョブ名を変更する場合はRulesetも更新する必要がある

## 関連資料

- [リポジトリ公開前監査](./repository-publication-audit.md)
- [Git履歴のメールアドレス公開方針](../research/git-history-email-publication.md)
- [ADR-0005: Windows向けバイナリを配布せずローカルでビルドする](../adr/0005-build-locally-without-distributing-windows-binaries.md)
