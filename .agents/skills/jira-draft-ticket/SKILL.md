---
name: jira-draft-ticket
description: Draft copy-ready Jira tickets using the project ticket policy. Use when the user asks to draft, write, prepare, or create the content for a Jira ticket, issue, task, story, bug, or spike, including requests that mention SCRUM tickets or the Jira ticket template.
---

# Jiraチケット本文案の作成

Jiraへ手動でコピーできるチケット本文案を、プロジェクト規約に沿って作成する。

## ワークフロー

1. `references/ticket-policy.md` を読む。
2. ワークスペースに `docs/develop/workflow/ticket.md`、`docs/develop/workflow/story-point.md`、`docs/templates/jira-ticket.md` があれば読み、リポジトリ固有の最新ルールを優先する。
3. ユーザーの依頼、関連コード、既存チケット情報を読み取り専用で確認する。
4. 内容に応じてIssue Typeを選び、選定理由を簡潔に説明する。
5. 目的、完了条件、対象外を基にStory Pointsを見積もる。
6. Summaryと本文案を作成する。不明な事実を推測で補わない。
7. 情報不足が完了条件、作業範囲、見積もりを大きく変える場合だけ、ユーザーへ確認する。
8. JiraへコピーしやすいMarkdownコードブロックで出力する。

## 出力形式

次の順で出力する。

1. Project Key
2. Issue Type
3. Summary
4. Story Points
5. Description

Descriptionには次の見出しを使用する。

```markdown
## Purpose

## As-Is

## To-Be

## Acceptance Criteria

## Out of Scope

## Progress

## Remarks

### Story Point Rationale

- Story Points:
- 作業量:
- 複雑さ:
- 不確実性:
- リスク:
- 相対比較:

### Additional Notes
```

Acceptance CriteriaとProgressはチェックリスト形式にする。該当事項がない項目は削除せず、`なし` と記載する。

## Story Pointの見積もり

- Story Pointの尺度と見積もりルールは、`docs/develop/workflow/story-point.md` をSSOTとして従う
- SSOTを参照できない場合は見積もりを推測せず、参照できないことを伝える
- 見積もり値と根拠を `Remarks > Story Point Rationale` に記載する
- 比較対象がない場合は、相対比較へ `比較可能な既存チケットなし` と記載する

## 記述規則

- Summaryは変更内容が分かる簡潔な日本語にする
- Purposeには作業内容ではなく、解決する課題と得たい価値を書く
- As-IsとTo-Beは対応関係が分かるようにする
- Acceptance Criteriaには外部から客観的に確認できる完了条件を書く
- Out of Scopeで今回扱わない事項を明示する
- Progressには完了までの作業内訳を書き、Jiraステータスの代わりにしない
- Story Pointの根拠は `Story Point Rationale` に書く
- 制約、判断理由、関連資料など、Story Point以外の補足は `Additional Notes` に書く
- 実装方法をAcceptance Criteriaへ混在させない
- 未着手の作業を完了済みとして記載しない

## 安全規則

- Jira、外部API、外部サービスへデータを送信しない
- チケットの作成、更新、遷移、削除を行わない
- APIトークンや認証情報を要求しない
- 秘密情報や個人情報を本文案へ含めない
- ユーザーが手動でJiraへコピーする前提で、本文案の生成だけを行う
