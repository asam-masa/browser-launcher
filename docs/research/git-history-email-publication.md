# Git履歴のメールアドレス公開方針

## 結論

現在のリポジトリはPrivateのまま維持します。将来のPublic版は、公開直前に監査した管理対象ファイルから新しいリポジトリを作り、1つの初期コミットから履歴を開始します。

この方針により、現在のGit履歴を破壊せず、過去の作者・コミッターメールを公開対象から除外します。Public版では、Firefox対応やマルチディスプレイ対応など、公開後の変更履歴を小さなコミットとPull Requestで蓄積します。

## Context

SCRUM-39の公開前監査で、GitHubの`noreply`ではない作者・コミッターメールをGit履歴から確認しました。SCRUM-45では、値を表示せずに2026年8月22日時点の`main`を再集計しました。

| 確認項目 | 結果 |
| --- | --- |
| 基準コミット | `966d46d` |
| コミット総数 | 114件 |
| 非`noreply`の作者メールを含むコミット | 68件 |
| 非`noreply`のコミッターメールを含むコミット | 68件 |
| `.mailmap` | なし |

Gitのコミットオブジェクトには作者とコミッターのメールが保存されます。今後の設定を変更しても、過去のコミットに記録されたメールは変わりません。

## Options

### 1. 現在の履歴を維持して公開する

現在のリポジトリをそのままPublicへ変更します。

利点は、コミット、Pull Request、参照を維持できることです。欠点は、過去のメールが公開されることです。個人メールを公開しない方針と両立しないため、採用しません。

### 2. 現在の履歴を書き換えて公開する

過去の作者・コミッターメールを`noreply`へ置き換え、リモート履歴を更新します。

利点は、変更の順序をおおむね維持できることです。欠点は、対象コミット以降のSHAが変わり、Force push、既存のPull Request、参照、ローカルブランチへ影響することです。古いコミットがGitHub上の参照やキャッシュに残る可能性もあり、メールの非公開を保証できません。影響が大きく目的に見合わないため、採用しません。

### 3. 新しいPublicリポジトリを作る

現在の管理対象ファイルを監査し、新しいリポジトリへ1つの初期コミットとして追加します。現在のリポジトリと履歴はPrivateのまま保管します。

利点は、既存履歴を変更せず、過去のメールを公開対象から除外できることです。欠点は、現在のコミットとPull Requestの履歴をPublic版へ引き継げないことです。公開目的は学習成果と今後の開発過程を示すことであり、過去の履歴維持より個人情報の保護を優先できるため、この案を採用します。

## `.mailmap`を採用しない理由

`.mailmap`は、作者・コミッターの名前とメールを正規化して表示するための対応表です。元のコミットオブジェクトは変更せず、元のメールをリポジトリから除去しません。そのため、表記統一には使用できますが、個人メールの秘匿には使用できません。

## 今後のコミット

今後のコミットでは、GitHubが提供するID付き`noreply`メールを使用します。2026年8月22日に、このリポジトリのローカルGit設定がID付き`noreply`形式であることを値を表示せずに確認しました。

リポジトリ単位の設定を優先し、グローバルGit設定へ影響させません。メールアドレスの値は、文書、チケット、Pull Request、レビュー、ログへ記録しません。

## Public版への移行条件

Public版の作成は後続チケットで扱います。少なくとも次を完了するまで、新しいリポジトリをPublicへ変更しません。

- Public版のリポジトリ名、Goモジュールパス、Private版の保管方法を決定する
- 公開対象の管理ファイルを確定する
- `.git`、ローカル設定、生成物を移行対象から除外する
- Gitleaksで移行対象と新しいGit履歴を検査する
- ID付き`noreply`メールで初期コミットを作成する
- READMEに、Privateで初期開発した成果を監査後にPublic版へ移したことを記載する
- ライセンスと`SECURITY.md`を含める
- Public化直後にGitHubのセキュリティ設定を有効にして確認する
- 移行元のPrivateリポジトリを変更または削除しない

## Consequences

- 現在のリポジトリはPublic化しない
- 現在の履歴を書き換えない
- Public版は現在のPull Requestとコミット履歴を引き継がない
- Public版の初期コミット以降で、レビュー可能な変更履歴を蓄積する
- 同じ所有者の下に同名のリポジトリを2つ作れないため、Public版の名前またはPrivate版の保管名を後続チケットで決定する
- Public版の作成とセキュリティ設定には、別途チケットと承認が必要になる

## References

- [GitHub: Setting your commit email address](https://docs.github.com/en/account-and-profile/how-tos/email-preferences/setting-your-commit-email-address)
- [GitHub: Email addresses reference](https://docs.github.com/en/account-and-profile/reference/email-addresses-reference)
- [GitHub: Changing a commit message](https://docs.github.com/en/pull-requests/how-tos/commit-changes/changing-a-commit-message)
- [Git: gitmailmap Documentation](https://git-scm.com/docs/gitmailmap)
