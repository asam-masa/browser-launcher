# Jiraチケットテンプレート

以下をJiraのチケット本文へコピーして使用します。文章は[日本語執筆ガイド](../develop/quality/japanese-writing.md)に従って記載します。JiraのStory Pointsフィールドには見積もった数値を入力し、その根拠を本文の `Story Point Rationale` に記載します。HTMLコメントは記載時の説明であり、チケットの内容として残す必要はありません。

```markdown
## Purpose

<!-- この作業を行う目的と、解決したい課題を記載する -->

## As-Is

<!-- 現在の状態と問題点を記載する -->

## To-Be

<!-- チケット完了後に実現している状態を記載する -->

## Acceptance Criteria

<!-- 外部から完了を確認できる条件を記載する -->

- [ ]
- [ ]
- [ ]

## Out of Scope

<!-- 今回対応しない事項を記載する。ない場合は「なし」と記載する -->

-

## Progress

<!-- 完了までに必要な作業単位を記載する。チケット自体の状態はJiraのステータスで管理する -->

- [ ]
- [ ]
- [ ]

## Remarks

### Story Point Rationale

<!-- Story Pointの数値と、作業量、複雑さ、不確実性、リスク、相対比較を記載する -->

- Story Points:
- 作業量:
- 複雑さ:
- 不確実性:
- リスク:
- 相対比較:

### Additional Notes

<!-- Story Point以外の制約、補足、判断理由、関連チケットや資料を記載する。ない場合は「なし」と記載する -->
```

## 記載上の注意

- `Purpose` には作業内容ではなく、作業が必要な理由を書く
- `As-Is` と `To-Be` は対応関係がわかるように書く
- `Acceptance Criteria` は、実装方法ではなく外部から確認できる完了条件を書く
- `Out of Scope` で今回扱わない事項を明確にし、作業範囲の拡大を防ぐ
- `Progress` は進捗率ではなく作業内訳をチェックリストで表す
- チケット全体の進行状態はJiraのステータスを正とする
- JiraのStory Pointsフィールドには数値だけを入力する
- `Story Point Rationale` にはStory Pointの数値と具体的な見積もり根拠を記載する
- `Additional Notes` にはStory Point以外の補足だけを記載する
