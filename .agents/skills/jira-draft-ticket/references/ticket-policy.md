# Jiraチケット作成ポリシー

## 既定値

| 設定 | 値 |
| --- | --- |
| Jiraサイト | `https://kurosahari.atlassian.net` |
| Project Key | `SCRUM` |
| Issue Type | 内容に応じて `Task`、`Story`、`Bug`、`Spike` から選択 |

## Issue Type

- `Story`: ユーザーや利用者へ価値を提供する
- `Bug`: 期待する振る舞いと実際の振る舞いが異なる
- `Task`: 技術・運用上必要だが、直接的なユーザー価値として表しにくい
- `Spike`: 実装方針を決めるための期限付き調査

## 本文

次の順序を維持する。

1. Purpose: 作業が必要な理由と得たい価値
2. As-Is: 現在の状態と具体的な問題
3. To-Be: 完了後に実現している状態
4. Acceptance Criteria: 外部から確認できる完了条件
5. Out of Scope: 今回対応しない事項
6. Progress: 完了までの作業内訳
7. Remarks: 制約、判断理由、関連資料

Acceptance CriteriaとProgressはチェックリストにする。ProgressはJiraステータスの代替にせず、作業内訳だけを示す。

Bugには再現手順、期待結果、実際の結果、環境、頻度、影響範囲を含める。Spikeには明らかにしたい問い、調査期限、成果物、調査後の意思決定を含める。

## Story Points

Story Pointの尺度と見積もりルールは、`docs/develop/workflow/story-point.md` をSSOTとして従う。参照できない場合は見積もりを推測しない。

## Remarks

Remarksを次の小見出しに分ける。

- `Story Point Rationale`: 見積もり値、作業量、複雑さ、不確実性、リスク、相対比較
- `Additional Notes`: 制約、判断理由、関連資料など、見積もり以外の補足
