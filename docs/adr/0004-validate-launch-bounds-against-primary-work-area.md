# ADR-0004: 物理座標でChrome起動条件を使用可能領域と比較する

- Status: Accepted
- Date: 2026-08-01
- Related: SCRUM-19

## Context

Browser Launcherは、利用者が論理ピクセルで入力した位置とサイズを使用して、Chromeウィンドウをプライマリディスプレイへ配置します。Chromeを起動する前に、要求したウィンドウ全体がタスクバーなどを除く使用可能領域内へ収まることを検証する必要があります。

Windowsの`GetMonitorInfo`が返す`rcWork`は、仮想スクリーン上の物理的な座標です。一方、利用者の入力値は表示倍率に依存しない論理ピクセルです。異なる座標系を直接比較すると、100%以外の表示倍率や境界値で誤った判定になります。

後続のChrome配置処理は、論理ピクセルを物理ピクセルへ変換して`SetWindowPos`へ渡します。事前検証と実際の配置で異なる変換や丸めを使用すると、検証に成功した要求が配置時に使用可能領域を超える可能性があります。

## Decision Drivers

- 事前検証と実際の配置で同じ座標と丸めを使用できること
- 100%以外の表示倍率でも境界を一貫して判定できること
- Windows固有のAPIとDPI認識状態をInfrastructureへ隔離できること
- 領域内判定をWindows APIなしで自動テストできること
- Linux上でもDomainとApplicationのテストおよび静的解析を実行できること
- Wailsプロセス全体のDPI認識状態へ不要な影響を与えないこと

## Considered Options

### Option 1: 使用可能領域を論理ピクセルへ変換する

Windowsから取得した使用可能領域を論理ピクセルへ変換し、利用者の入力値と比較します。

利点:

- 利用者の入力値をそのまま比較できる
- Domainへ渡す値を論理ピクセルに統一できる

欠点:

- 物理座標から論理座標へ変換するときに端数が発生する
- 配置時に再び物理座標へ変換すると、境界がずれる可能性がある
- 事前検証と`SetWindowPos`へ渡す値が一致しない可能性がある

### Option 2: 入力値を物理ピクセルへ変換する

利用者の入力値を物理ピクセルへ変換し、Windowsから取得した使用可能領域と同じ座標系で比較します。

利点:

- Windowsの`rcWork`を逆変換せずに比較できる
- 配置時と同じ変換および丸めを使用できる
- 既存のWindows実機検証と同じ方式を使用できる

欠点:

- ApplicationがDPIを使用した変換を調整する必要がある
- Domainへ渡す2つの矩形が同じ座標系であることを契約として維持する必要がある

### Option 3: Infrastructureで範囲検証まで行う

Windows APIから使用可能領域を取得する処理と、要求矩形の範囲検証をInfrastructure内で行います。

利点:

- Windows固有処理の近くで検証が完結する

欠点:

- 使用可能領域内へ収める規則がWindows固有コードへ入り込む
- DomainとApplicationのUnit Testで境界値を検証しにくい
- Windows以外の環境で重要な規則を検証できない

## Decision

Option 2を採用します。

Infrastructureは、プライマリディスプレイのモニター領域、使用可能領域、DPIを物理座標で取得します。Applicationは利用者が入力した論理ピクセルを物理座標へ変換します。Domainは同じ座標系で表された要求矩形と使用可能領域を比較し、要求矩形の4辺が使用可能領域内に収まるか判定します。

論理ピクセルから物理ピクセルへの変換には`DPI / 96`を使用します。端数は0から遠ざかる方向へ丸め、後続のChrome配置処理でも同じ変換規則を使用します。

Windows Infrastructureは、APIを呼び出すgoroutineをOSスレッドへ固定します。スレッドのDPI認識状態をPer-Monitor DPI Aware V2へ一時的に変更し、処理終了前に以前の状態へ復元します。プロセス全体のDPI認識状態は変更しません。

プライマリディスプレイは、仮想スクリーンの原点を指定した`MonitorFromPoint`で取得します。同じ`HMONITOR`を`GetMonitorInfo`と`GetDpiForMonitor`へ渡し、`rcMonitor`、`rcWork`、`MDT_EFFECTIVE_DPI`を取得します。これにより、使用可能領域と表示倍率の取得対象を同じモニターに固定します。値はキャッシュせず、検証ごとに取得します。

Microsoftは、Per-Monitor DPI awareなスレッドでは、`GetDpiForMonitor`ではなく`GetDpiForWindow`の使用を推奨しています。ただし、起動条件の検証時点では対象Chromeの`HWND`が存在しません。起動前の事前検証では対象モニターを明示できる`GetDpiForMonitor`を使用し、Windows実機で取得値を検証します。後続のChrome配置処理では、起動後に取得したChromeの`HWND`を`GetDpiForWindow`へ渡し、実際の配置先のDPIと使用可能領域で最終検証します。

最終検証は、Chromeを対象モニターへサイズ変更せずに移動し、矩形、DPI、モニター、使用可能領域が安定した後に行います。Infrastructureは、対象ウィンドウのDPI、モニター領域、使用可能領域をApplicationが定義する最終矩形解決処理へ渡します。Applicationは起動前と同じ変換・丸め規則で論理的な要求を物理矩形へ変換し、Domainの`Bounds`で使用可能領域との内包関係を判定します。検証に失敗した場合、Infrastructureは最終的な位置とサイズを適用しません。

二段階配置の途中では、呼び出したOSスレッドのDPI認識状態を維持する必要があります。そのため、Applicationは変換と判定の方針をコールバックとして定義し、Infrastructureは移動後の安定を確認した時点でこの方針を呼び出します。範囲判定をInfrastructureへ実装せず、二段階配置のOS操作もApplicationへ分割しないことで、依存方向とOSスレッドの安全性を両立します。

ApplicationはInfrastructureの契約を利用側で定義します。Windows以外の実装は未対応エラーを返し、製品機能を提供しません。これにより、Linux上でもパッケージ全体をコンパイルし、DomainとApplicationのUnit Testを実行できる状態を維持します。

## Consequences

- 事前検証とChrome配置処理で、論理ピクセルから物理ピクセルへの変換を共有する必要がある
- Domainは座標の単位を変換せず、同じ座標系にある矩形の内包関係だけを扱う
- Applicationは使用可能領域を取得できない場合と入力値が範囲外の場合を区別する必要がある
- Wails AdapterはInfrastructureの詳細を隠し、利用者が再試行できるメッセージへ変換する必要がある
- Windows APIの成否とDPI認識状態の復元はWindows 11でIntegration TestまたはManual Testを行う必要がある
- 事前検証はChromeの実際の起動先を保証しないため、起動後の配置処理で`GetDpiForWindow`を使用した最終検証が必要になる
- 最初のモニター移動後に最終検証が失敗した場合、ウィンドウは部分的に移動した状態となる。自動的に元の位置へ戻さず、処理段階、部分変更、最後の実測矩形、入力項目ごとの問題を結果として返す
- マルチディスプレイ対応では、対象モニターの選択とDPI取得方法を別のADRで見直す必要がある

## Validation

- [x] DPI 96、120、144で論理ピクセルを物理ピクセルへ変換できる
- [x] 使用可能領域の4辺と一致する要求を正常と判定できる
- [x] 各辺を1物理ピクセル超える要求を拒否できる
- [x] Windows 11でプライマリディスプレイの`rcMonitor`、`rcWork`、DPIを取得できる
- [x] Windows 11でプライマリディスプレイを変更した後も、対象モニターのDPIを取得できる
- [x] DPI認識状態を処理後に復元できる
- [x] Windows 11で使用可能領域の境界値を検証できる
- [x] 移動後のDPIとモニター原点を使用して、論理的な要求を最終的な物理矩形へ変換できる
- [x] 最終的な要求が使用可能領域を超える場合、最終配置を実行しないことをUnit Testで確認できる
- [x] Windows 11の100%と150%で、最終検証後に要求した物理矩形へ配置できる
- [x] Windows 11で使用可能領域を超える要求に対して、最終配置を実行しない

## References

- [Microsoft Learn: MONITORINFO structure](https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-monitorinfo)
- [Microsoft Learn: SetThreadDpiAwarenessContext function](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setthreaddpiawarenesscontext)
- [Microsoft Learn: GetDpiForMonitor function](https://learn.microsoft.com/en-us/windows/win32/api/shellscalingapi/nf-shellscalingapi-getdpiformonitor)
- [Microsoft Learn: GetDpiForWindow function](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getdpiforwindow)
- [ADR-0001: WindowsでChromeウィンドウを起動・配置する](./0001-control-chrome-window-on-windows.md)
