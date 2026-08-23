# Public版リポジトリ移行

## 状態

移行作業中です。Private版は改名済みです。Public版の作成、初期コミット、Public化は未実施です。

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

検出0件は秘密情報が存在しないことを保証しません。Public版の初期コミットを作成した後、Git履歴を対象に再検査します。

### 移行後検証

移行実施後に、値や検出内容などの秘密情報を記録せず、次の結果を追記します。

- 基準コミットと公開対象ファイル
- Gitleaksによる移行対象とPublic版Git履歴の検査結果
- ローカル検証とLinux・Windows CIの結果
- ID付き`noreply`メールの確認結果
- Public版の可視性と公開表示
- Private vulnerability reporting、Secret scanning、Push protection、Dependabot、Rulesetの設定結果
- ForkからのPull Requestに対するActionsの承認設定
- 未確認事項と残存リスク

## 関連資料

- [リポジトリ公開前監査](./repository-publication-audit.md)
- [Git履歴のメールアドレス公開方針](../research/git-history-email-publication.md)
- [ADR-0005: Windows向けバイナリを配布せずローカルでビルドする](../adr/0005-build-locally-without-distributing-windows-binaries.md)
