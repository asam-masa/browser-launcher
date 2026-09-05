# 依存関係の更新

## 目的

依存関係の更新候補を定期的に確認し、既存のレビューとCIを使用して安全に取り込みます。通常のversion updatesにはDependabotを使用し、更新PRは自動マージしません。

## 対象とスケジュール

Dependabotは、毎週月曜日の09:00（Asia/Tokyo）に次の対象を確認します。

| パッケージエコシステム | 対象ディレクトリ |
| --- | --- |
| Go Modules | `/`、`/scripts/spike/wails-communication` |
| npm | `/`、`/frontend`、`/scripts/spike/wails-communication/frontend` |
| GitHub Actions | `/` |

設定は[`.github/dependabot.yml`](../../../.github/dependabot.yml)で管理します。

## PRのグループ化

minorとpatchのversion updatesは、Go Modules、npm、GitHub Actionsの各パッケージエコシステムでグループ化します。majorはグループに含めず、個別PRとして確認します。

グループPRに問題がある場合は、失敗した検証と差分から原因を切り分けます。複数の依存関係が原因候補になる場合は、一部の更新を除外する設定を恒久的に追加する前に、個別のチケットで対応方針を決めます。

## 更新PRの確認

更新PRでは、次を確認します。

1. リリースノートと変更内容を確認する
2. manifestとlock file以外に意図しない差分がないことを確認する
3. LinuxとWindowsのCIが成功することを確認する
4. 製品動作に影響する更新では、必要なWindows実機検証を行う
5. majorでは、破壊的変更と移行手順を個別に確認する

GitHub Actionsの更新では、ActionがコミットSHAで固定されていることと、同じ行のバージョンコメントが更新内容と一致することも確認します。

Dependabotが作成したPRは、検証結果を確認してから手動でマージします。自動マージとmajorの自動承認は使用しません。

## セキュリティ更新との関係

Dependabot alertsとDependabot security updatesは継続します。`.github/dependabot.yml`のグループ規則は`applies-to: version-updates`で通常のversion updatesだけを対象にし、security updatesを無効化しません。

セキュリティ更新は通常の週次更新を待たず、影響と緊急度を確認して対応します。

## 対象外

バインディング生成コマンド内で固定しているWails CLI v2.12.0は、Dependabotの対象ではありません。Wailsを更新する場合は、生成物、Goモジュール、フロントエンド、Windows実機への影響を一つのチケットで確認します。

private registry、Renovate、自動マージ、依存関係更新専用の資格情報は、現在の運用へ導入しません。

## 設定変更後の確認

設定変更時は、次を確認します。

- YAMLとして解析できること
- パッケージエコシステム、対象ディレクトリ、スケジュール、グループ規則が意図した値であること
- Markdown Lintとローカルリンク検査が成功すること
- LinuxとWindowsのCIが成功すること

default branchへのマージ後は、GitHubのリポジトリ設定にDependabot version updatesの対象が表示され、設定エラーが報告されていないことを確認します。更新PRが作成された場合は、グループ、majorの分離、GitHub ActionsのSHA固定とバージョンコメントを確認します。
