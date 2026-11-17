# QuickNetScan について

## 1. アプリケーション情報

- **アプリケーション名**: QuickNetScan
- **日本語名称**: クイック・ネットスキャン
- **最新バージョン**: v1.0.0
- **リリース日**: 2026-11-17
- **制作者**: maktak-105 (GitHub: https://github.com/maktak-105)
- **ライセンス**: MIT License

## 2. 技術スタック

| コンポーネント | 技術 |
| --- | --- |
| スキャンエンジン | Go 1.22+ (Goroutine 超並行ソケット/ARP走査) |
| フロントエンド | WebAssembly (Go WASM) + HTML5 / CSS3 (Quick共通ダークテーマ) |
| バイナリ形態 | 単一静的実行ファイル (Go `embed` による完全内包) |
| CI/CD | GitHub Actions (Windowsランナー上での自動ビルド・完全性検証) |
