# リポジトリ公開前監査

## 結論

現在のリポジトリはPrivateのまま維持します。秘密情報の検査では検出がありませんでしたが、Git履歴にGitHubの`noreply`ではない作者・コミッターメールが含まれています。過去のメールを公開対象から除外するため、将来のPublic版は監査済みの管理対象ファイルから新しいリポジトリとして作成します。

本監査では公開可否だけを評価しました。リポジトリの可視性、Git履歴、リモートブランチは変更していません。

## 対象

- 現在の管理対象ファイル
- すべてのローカルGit参照から到達できる履歴
- `AGENTS.md`と`.agents/skills/`
- Jira連携に関する文書と実装
- 直接依存関係のライセンス
- GitHubのセキュリティ設定とActions設定

## 監査結果

### 秘密情報

検査結果を再現できるよう、使用するGitleaksをv8.29.1へ固定しました。[Gitleaks v8.29.1の公式リリース](https://github.com/gitleaks/gitleaks/releases/tag/v8.29.1)からLinux x64配布物を一時ディレクトリへ取得し、公式チェックサムと一致することを確認してから検査しました。

| 確認項目 | 結果 | 根拠 |
| --- | --- | --- |
| GitleaksによるGit履歴全体の検査 | 検出0件 | `gitleaks git --redact=100 --log-opts=--all`が終了コード0で完了 |
| Gitleaksによる作業ツリーの検査 | 検出0件 | `gitleaks dir --redact=100 .`が終了コード0で完了 |
| 代表的なトークン、秘密鍵、認証ヘッダーの補助検索 | 検出0件 | 値を出力しない正規表現検索を全履歴へ実施 |
| 秘密情報を示すファイル名 | 検出0件 | 管理対象ファイルの名前を確認 |
| `.env`と秘密鍵の除外 | 対応済み | `.gitignore`で派生ファイルと主要な秘密鍵形式を除外 |

検出0件は秘密情報が存在しないことを保証しません。未知の形式、分割された値、画像や圧縮ファイル内の値は検出できない可能性があります。

### 個人情報と内部情報

| 確認項目 | 結果 | 公開判断 |
| --- | --- | --- |
| Git履歴の作者・コミッターメール | 2026年8月22日の`main`（`966d46d`）で、GitHubの`noreply`ではない作者・コミッターメールをそれぞれ68コミットで確認 | 現在のリポジトリをPrivateで維持し、新しいPublicリポジトリへ履歴を移行しない |
| Linuxのホームディレクトリ | 該当なし | 公開可能 |
| Windowsのユーザーディレクトリ | テスト用のパスを3ファイルで確認 | 実在利用者の情報ではないため公開可能 |
| JiraサイトURL | 5ファイルで確認 | 認証情報ではなく、接続先を固定する安全要件と利用手順に必要なため公開可能。ただし、Jiraテナント名は公開される |

SCRUM-45で[Git履歴のメールアドレス公開方針](../research/git-history-email-publication.md)を決定しました。現在のGit履歴は書き換えず、Public版へ移行しません。今後のコミットにはGitHubが提供するID付き`noreply`メールを使用します。

### AIエージェント関連ファイル

`AGENTS.md`と`.agents/skills/`には、開発方針、レビュー手順、Jiraチケットの作成・参照手順が含まれています。APIトークンとアカウントメールの値は含まれていません。

`read-jira-ticket` Skillは接続先とProject Keyを固定し、環境変数から認証情報を取得します。接続先の公開を許容する一方、環境変数の値、APIレスポンス、個人情報を管理対象へ追加しない運用を継続します。

### ライセンス

本リポジトリにはMIT Licenseを採用しました。学習目的のソースコードを簡潔な条件で閲覧、利用、変更でき、再利用時に著作権表示とライセンス表示を求められるためです。

直接依存関係のパッケージメタデータまたは同梱ライセンスを確認しました。

| 対象 | 確認したライセンス |
| --- | --- |
| Wails v2.12.0 | MIT |
| `golang.org/x/sys` v0.30.0 | BSD 3-Clause |
| React、Vite、Vitest、Testing Libraryなどの直接npm依存 | MIT |
| TypeScript 5.9.3 | Apache-2.0 |

本プロジェクトはバイナリを配布しません。将来配布する場合は、間接依存を含むライセンス表示と配布条件を別途監査します。

### 脆弱性の報告

`SECURITY.md`に、脆弱性を公開Issueへ記載しない方針を定義しました。[GitHubのPrivate vulnerability reporting](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository)はPublicリポジトリの管理者が設定する機能です。可視性を変更した直後に有効化し、報告経路を確認するまでPublic化を完了扱いにしません。

2026年8月16日時点ではリポジトリがPrivateであるため、Private vulnerability reportingのAPIによる状態確認は404で終了しました。現在は非公開の報告窓口を提供していません。

## GitHub設定チェックリスト

### 新しいPublicリポジトリの作成前

- [x] リポジトリがPrivateであることを確認する
- [x] GitHub Actionsの既定権限が`read`であることを確認する
- [x] CIの`GITHUB_TOKEN`権限が`contents: read`に限定されていることを確認する
- [x] GitleaksでGit履歴全体を検査する
- [x] ライセンスとセキュリティポリシーを追加する
- [x] 現在のGit履歴をPublic版へ移行しない方針を決定する
- [x] 今後のコミットでID付き`noreply`メールを使用する方針を決定する
- [ ] Public版へ移行する最終ファイルをGitleaksで再検査する
- [ ] Public版の新しいGit履歴をGitleaksで検査する
- [ ] Public化について依頼者の個別承認を得る

### 可視性変更直後・公開完了判定前

- [ ] Private vulnerability reportingを有効にする
- [ ] `SECURITY.md`の「Report a vulnerability」が利用できることを確認する
- [ ] Secret scanningとPush protectionの利用可否を確認し、利用可能な機能を有効にする
- [ ] Dependabot alertsとDependabot security updatesを有効にする
- [ ] `main`へ必要なRulesetまたはBranch protectionを設定する
- [ ] ForkからのPull Requestに対するActionsの承認設定を確認する
- [ ] Public表示から秘密情報、個人情報、不要な内部情報が見えないことを再確認する

Private状態では、現在のプランでBranch protection APIがHTTP 403となったため、設定状態を確認できませんでした。Public化後にRulesetまたはBranch protectionを設定し、CIを必須チェックにするかを判断します。

## 未確認事項と残存リスク

- Public版の作成と公開後確認は[Public版リポジトリ移行](./public-repository-migration.md)で扱う
- Private vulnerability reporting、Secret scanning、Push protection、Dependabotの公開後設定は未実施
- Gitleaksで検出できない形式の秘密情報が存在する可能性がある
- 間接依存を含む完全なライセンス監査は未実施
- 実際のPublic表示とForkからのCI実行は未確認

## Related

- SCRUM-38: Windows向けビルド・配布方針を決定する
- SCRUM-39: リポジトリ公開前のセキュリティ・ライセンス監査を実施する
- SCRUM-45: Git履歴のメールアドレス公開方針を決定する
