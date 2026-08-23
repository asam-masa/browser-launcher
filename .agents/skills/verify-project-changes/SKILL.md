---
name: verify-project-changes
description: Select and run the checks required by Git changes in this repository, then summarize verified results, omissions, and residual risks without fixing failures. Use when the user asks to test, validate, verify, or prepare Result evidence for working-tree, staged, branch, commit, or pull-request changes.
---

# プロジェクト変更検証

Git差分と現在の設定から必要な検証を選択し、確認できた結果だけをPRへ転記可能な形式で報告する。検証失敗を修正しない。

## ワークフロー

### 1. 規約と検証範囲を確定する

`AGENTS.md`、`docs/develop/quality/testing.md`、`docs/develop/workflow/pull-request-review.md`を確認する。比較元は、依頼者の指定を優先し、指定がなければ次の順で決める。

1. 指定されたPR、コミット、Git範囲
2. 現在のブランチとBaseブランチの共通祖先からの差分
3. Baseブランチとの差分がない場合は現在のHEAD

比較元を決めた後、比較元からHEADまでの差分へ、ステージ済み、未ステージ、未追跡を含む作業ツリーの変更を重ねて検証対象にする。依頼者がコミットやステージ済み差分などへ範囲を限定した場合だけ、指定外の変更を対象外として明記する。比較元を一意に決められない場合は、結果が変わり得るため依頼者へ確認する。

### 2. 現在有効なコマンドを確認する

README、`go.mod`、`package.json`、ロックファイル、CI、ビルド設定を確認する。これらが検証対象に含まれる場合は、変更後の内容からコマンド、ツール、必要バージョンを判断する。文書の例より、現在のマニフェストと設定を優先する。存在しないlint、テスト、ビルドを推測で追加しない。

### 3. 変更内容から検証を選択する

| 変更 | 選択する検証 |
| --- | --- |
| GoコードまたはGo設定 | `go test ./...`、`go vet ./...`、`go build ./...` |
| `go.mod`または`go.sum` | Go検証に加えて`go mod tidy -diff` |
| `frontend/`のソースまたは設定 | `npm test`、`npm run build`を`frontend`で実行 |
| Markdown | 検証範囲ごとの`git diff --check`、未追跡Markdownの空白検査、リポジトリ全体のローカルリンク検査 |
| `.agents/skills/` | 対象Skillごとに`quick_validate.py`、リポジトリ全体のローカルリンク検査 |
| Windows固有コード | 該当するGo・Frontend検証に加えて、Windows上のテストとビルド |
| PowerShell | Windows上の組み込みParserによる構文検査。実行による確認が必要な場合は別途承認を得る |
| UI表示、操作、Wails境界 | 該当する自動検証に加えて、Windows上の実機確認 |

コミット済み差分、ステージ済み差分、未ステージ差分を含む場合は、該当する検査をそれぞれ実行する。`<comparison-range>`には、手順1で確定した比較元からHEADまでの範囲を指定する。

```bash
git diff --check <comparison-range>
git diff --cached --check
git diff --check
```

`git diff --check`は未追跡ファイルを検査しないため、未追跡Markdownは`rg -n '[ \t]+$' <files>`で末尾空白を検査する。この`rg`の終了コード1は、該当なしとして成功に分類する。MarkdownまたはSkillを変更した場合は、次をリポジトリルートで実行する。

```bash
python3 -B .agents/skills/check-doc-consistency/scripts/check_markdown_links.py .
```

複合変更では該当する検証を統合し、同じコマンドを重複して実行しない。Windows固有の検証は、通常のGo・Frontend検証を置き換えずに追加する。生成物だけの差分は生成元を追跡し、生成物だけを根拠に検証を選択しない。

### 4. 実行前に影響を確認する

選択したコマンドと実行環境を提示する。外部通信、依存関係の取得、リポジトリ内への生成、システム設定変更、GUI操作が発生する場合は、影響を説明して依頼者の承認を得る。

依存関係が不足している場合は、自動でインストールしない。Frontendビルドは`frontend/dist`を変更し得るため、依頼者の承認後に次のような方法でリポジトリ外へ出力する。承認を得られない場合は、未実施として理由を記録する。

```bash
verification_output_dir=$(mktemp -d)
npm --prefix frontend run build -- --outDir "$verification_output_dir"
```

作成した一時ディレクトリのパスを結果へ記録する。削除する場合は、対象と影響を示して依頼者の承認を得る。

### 5. 検証を実行する

各コマンドの終了まで確認する。途中経過や起動だけを成功として扱わない。失敗後も独立して安全に実行できる検証は継続し、複数の結果を収集する。

Windows固有の検証は、実際のWindows環境で完了した場合だけ成功とする。Linux上のクロスコンパイルやコード確認を、Windows実機確認の代替として記録しない。UI表示、利用者操作、Wails境界の振る舞いが変わる場合は、対象操作のWindows実機確認を追加する。手動確認は、依頼者から確認した操作と結果だけを記録する。

PowerShellを変更した場合は、Windows上で変更対象ごとに次の構文検査を実行する。`<changed-script.ps1>`は、検証対象として確認したパスへ置き換える。Parserはスクリプトを実行しない。

```powershell
$tokens = $null
$errors = $null
[System.Management.Automation.Language.Parser]::ParseFile(
    (Resolve-Path '<changed-script.ps1>'),
    [ref]$tokens,
    [ref]$errors
) > $null
if ($errors.Count -gt 0) {
    $errors.Message
    exit 1
}
```

スクリプトの実行が必要な場合は、生成物、外部通信、システムへの影響を確認し、依頼者の承認を得る。構文検査だけを、実行結果の確認として扱わない。

### 6. 結果を分類する

各検証を次のいずれかへ分類する。

- `成功`: コマンドまたは手動確認が完了し、期待する終了結果を確認した
- `失敗`: コマンドが失敗した、または期待結果と異なった
- `未実施`: 必要だが環境、承認、依存関係などの理由で実行していない
- `対象外`: 差分から必要がないと判断できる

未実施と対象外には理由を付ける。失敗を対象外へ変更せず、実行していない検証を成功と記載しない。

## 出力形式

最初に全体結果を示し、続けてPRの`Result`へ転記できる表を出力する。

```markdown
## Verification Result

| Category | Check | Environment | Status | Evidence / Reason |
| --- | --- | --- | --- | --- |
| テスト | `go test ./...` | Linux | 成功 | 終了コード0 |
| ビルド | Windows実機ビルド | Windows | 未実施 | Windows環境を使用していないため |

## Unverified

- 未実施の検証と理由

## Residual Risks

- 未実施または自動化できない検証によって残るリスク。なければ`なし`
```

コマンド、環境、実際の結果を省略しない。失敗した場合は、安全に確認できた最小限のエラー要約を添え、秘密情報や大量のログを転載しない。

## 安全規則

- ソース、設定、文書、Git状態を変更しない。
- 検証失敗を自動修正しない。
- 依存関係を追加、更新、削除しない。
- Git履歴、ブランチ、インデックス、作業ツリーを変更しない。
- Windowsの表示設定、レジストリ、ブラウザ設定、その他のシステム設定を変更しない。
- テストが外部サービスや利用者データへ作用する場合は実行しない。
- 秘密情報、個人情報、認証情報をコマンドや報告へ含めない。
- コードレビュー、敵対的レビュー、PR作成を同時に実施しない。
