# QuickNetScan 仕様書

## 1. アプリ概要

- **名称**: QuickNetScan（クイック・ネットスキャン）
- **目的**: 自宅LANや社内LANのIPレンジを、Goの得意な並行処理（Goroutine）で数百スレッド同時に一瞬でスキャンし、生きている機器のIP・MACアドレス・空いているポート・応答速度を即座に一覧表示する単一EXE。
- **対象OS**: Windows 10 / 11 (64-bit)
- **実装**: Go (Goroutine並行スキャンエンジン) + WebAssembly (WASM) / HTML / CSS
- **配布形態**: GitHub Releases の ZIP（単一EXEのフラット構成）
- **バージョン**: v1.0.0

## 2. アーキテクチャ

```text
[WASM / HTML / CSS]  ←WebSocket / WebMessage / Local API→  [Go Host & Goroutine Engine]
```

- `internal/engine/`: 並行スキャンエンジン本体。ARPテーブル解決、ICMP Ping/TCP Connect並行スキャン、ポート走査、応答時間計測。GUI非依存でCLIからも共有
- `cmd/quicknetscan/`: GUI版エントリーポイント。ローカルホスト・WebView2またはブラウザ連携、埋め込みリソース（Go `embed`）管理
- `cmd/quicknetscan-cli/`: CLI版エントリーポイント
- `web/cmd/wasm/`: WebAssembly側のGoロジック。フロントエンドのリアクティブ状態管理
- `web/static/`: Quickシリーズ共通ダークUI（HTML/CSS）

## 3. 画面構成

| 領域 | 内容 |
| --- | --- |
| ヘッダー | タイトル（QuickNetScan）、言語切替ピルボタン（日/英）、バージョン情報モーダル起動、エクスポートボタン |
| コントロールパネル | IPレンジ入力（自動検出/CIDR/範囲）、スキャンポート設定（プリセット/カスタム）、並行数スライダー（Goroutine数）、スキャン開始/停止ボタン |
| メインテーブル | 検出デバイス一覧（状態、IPアドレス、MACアドレス、ベンダー、ホスト名、開放ポート、応答速度/RTT） |
| ステータスバー | スキャン進捗バー、検出台数、スキャン対象総数、経過時間 |

## 4. 機能一覧

1. **IPレンジ自動検出 & スキャン**
   - 自身のネットワークアダプターからサブネット（CIDR）を自動検出
   - 任意のIP範囲（例: 192.168.1.1 - 192.168.1.254）の指定スキャン
2. **Goroutine による超並行スキャン**
   - 数百のGoroutineを同時稼働させ、256台〜数千台のIPを数秒で走査
3. **機器情報の高速取得**
   - ARPキャッシュおよび高速Pingによる生死判定
   - MACアドレスとOUIベンダー名の解決
   - 代表的ポート（Web: 80, 443 / SSH: 22 / SMB: 445 / RDP: 3389 等）の高速ポートスキャン
4. **結果のエクスポート**
   - CSV / JSON 形式でのスキャン結果書き出し
5. **Quick共通ダークUI & 多言語対応**
   - 日本語 / 英語のワンクリック切り替え
   - 作者情報・技術スタックを表示するモーダル
