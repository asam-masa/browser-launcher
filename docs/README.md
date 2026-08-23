# ドキュメント

## プロダクト

- [プロダクト要件](./product/requirements.md)

## 開発

- [開発ガイド](./develop/README.md)

## 技術調査

- [npm依存関係の脆弱性調査](./research/npm-dependency-vulnerability.md)
- [Windows 11におけるChromeの起動と配置](./research/windows-chrome-launch.md)
- [Windows 11におけるChromeウィンドウの追跡](./research/windows-chrome-window-tracking.md)
- [デスクトップアプリケーション基盤とGo・React連携方式](./research/go-react-desktop-foundation.md)
- [Git履歴のメールアドレス公開方針](./research/git-history-email-publication.md)

## 検証記録

- [MVP受入テスト](./verification/mvp-acceptance-test.md)
- [リポジトリ公開前監査](./verification/repository-publication-audit.md)
- [Public版リポジトリ移行](./verification/public-repository-migration.md)

## Architecture Decision Records

- [ADR-0001: WindowsでChromeウィンドウを起動・配置する](./adr/0001-control-chrome-window-on-windows.md)
- [ADR-0002: Chromeウィンドウのライフサイクルを追跡する](./adr/0002-track-chrome-window-lifecycle.md)
- [ADR-0003: デスクトップアプリケーション基盤にWails v2を使用する](./adr/0003-use-wails-v2-for-desktop-foundation.md)
- [ADR-0004: 物理座標でChrome起動条件を使用可能領域と比較する](./adr/0004-validate-launch-bounds-against-primary-work-area.md)
- [ADR-0005: Windows向けバイナリを配布せずローカルでビルドする](./adr/0005-build-locally-without-distributing-windows-binaries.md)
