# ADR-0006: 依存関係のversion updatesにDependabotを使用する

- Status: Accepted
- Date: 2026-09-05
- Related: SCRUM-61

## Context

Browser Launcherは、Go Modules、npm、GitHub Actionsの依存関係を使用しています。依存関係の更新候補を継続して把握するには、更新の検出、Pull Request（以下、PR）の作成、実行時の権限を管理する必要があります。

このリポジトリはpublicであり、個人開発と学習を目的としています。更新作業の負担を抑えつつ、GitHub上の既存のCIとレビュー手順を利用できる方法を選択します。Dependabot alertsとDependabot security updatesは継続し、本ADRでは通常のversion updatesを扱います。

## Decision Drivers

- GitHub上の設定だけで運用できること
- 追加の資格情報や常時実行環境を管理しないこと
- Go Modules、npm、GitHub Actionsを更新できること
- minorとpatchの更新をまとめて、PRとCIの回数を抑えられること
- majorの更新を個別に確認できること
- Dependabot alertsとsecurity updatesを維持できること
- 個人開発に見合う単純な運用であること

## Considered Options

### Option 1: Dependabot version updatesを使用する

GitHubの標準機能を`.github/dependabot.yml`で設定します。

利点:

- GitHub App、Personal Access Token、セルフホスト環境を追加せずに導入できる
- 更新PRを既存のGitHub Actionsとレビュー手順で検証できる
- パッケージエコシステムごとに更新対象、スケジュール、グループを設定できる

欠点:

- Renovateと比較すると、更新ルールやPR作成方法のカスタマイズ範囲が狭い
- 設定がdefault branchへマージされるまで、GitHubによる認識を確認できない

### Option 2: RenovateのGitHub Appを使用する

Renovateが提供するホスト型GitHub Appをリポジトリへ導入します。

利点:

- 更新のグループ化、スケジュール、ルールを細かく設定できる
- Renovateの実行環境を自分で保守する必要がない

欠点:

- GitHub Appへリポジトリのコード、PR、workflowなどに対する広い権限を付与する必要がある
- 現在の単純な更新方針に対して、設定と機能の範囲が広い

### Option 3: Renovateをセルフホストする

自分でRenovateの実行環境と資格情報を管理します。

利点:

- 実行環境、バージョン、起動条件を自分で制御できる
- GitHub Appに処理を委ねずに運用できる

欠点:

- 実行環境、更新、監視、資格情報を継続して管理する必要がある
- 個人開発の依存関係更新に対して運用負荷が大きい

## Decision

Option 1を採用し、Dependabot version updatesを使用します。RenovateのGitHub Appとセルフホスト版は導入しません。

Go Modules、npm、GitHub Actionsを毎週月曜日の09:00（Asia/Tokyo）に確認します。Go ModulesはルートとWails通信spike、npmはルート、製品frontend、Wails通信spikeのfrontend、GitHub Actionsはリポジトリのworkflowを対象にします。

minorとpatchのversion updatesはパッケージエコシステムごとにグループ化します。majorのversion updatesはグループへ含めず、個別PRとして確認します。自動マージは使用しません。

Dependabot alertsとDependabot security updatesは無効化しません。version updatesのグループ規則は`applies-to: version-updates`で限定し、security updatesの既存の動作を変更しません。

GitHub ActionsはコミットSHAによる固定を維持します。DependabotがActionを更新する場合も、同じ行のバージョンコメントを含む差分をレビューします。

Wails CLIのバインディング生成コマンドは、依存関係マニフェストではなくコマンド内でv2.12.0を指定しています。このバージョンは本設定の自動更新対象にせず、Wails更新を扱うチケットで変更します。

## Consequences

- 通常の依存関係更新候補がGitHub上のPRとして定期的に提示される
- minorとpatchをまとめることで、PR数とCI実行回数を抑えられる
- majorは影響を個別に判断できる
- 更新PRのマージ判断と検証は引き続き開発者が行う必要がある
- 複数の更新をまとめたPRでは、不具合が生じた依存関係の特定に追加の切り分けが必要になる場合がある
- GitHubのDependabot実行環境と対応エコシステムに依存する
- より複雑な更新規則が必要になった場合は、Renovateを含めて本判断を見直す

## Validation

- [x] `.github/dependabot.yml`が有効なYAMLである
- [x] Go Modules、npm、GitHub Actionsの全対象ディレクトリが設定されている
- [x] 毎週月曜日の09:00（Asia/Tokyo）が設定されている
- [x] minorとpatchだけが各パッケージエコシステムのグループ対象である
- [x] majorが個別PRの対象として残っている
- [x] Markdown Lintとローカルリンク検査が成功する
- [ ] LinuxとWindowsのCIが成功する
- [ ] default branchへのマージ後にGitHubがDependabot設定を認識する
- [ ] GitHub Actionsの更新PRでSHA固定とバージョンコメントが維持される

最後の2項目は、設定がdefault branchへマージされ、該当する更新候補が検出された後に確認します。

## References

- [GitHub Docs: Dependabotオプションリファレンス](https://docs.github.com/en/code-security/reference/supply-chain-security/dependabot-options-reference)
- [GitHub Docs: Dependabot version updatesのPR作成を最適化する](https://docs.github.com/en/code-security/tutorials/secure-your-dependencies/optimizing-pr-creation-version-updates)
- [GitHub Docs: GitHub ActionsをDependabotで更新する](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/auto-update-actions)
- [Renovate Docs: Installing & Onboarding](https://docs.renovatebot.com/getting-started/installing-onboarding/)
- [Renovate Docs: Security and Permissions](https://docs.renovatebot.com/security-and-permissions/)
