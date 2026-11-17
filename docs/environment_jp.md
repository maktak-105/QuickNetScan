# QuickNetScan 開発環境・ビルド手順

## 1. 前提環境

- **OS**: Windows 10 (64-bit) または Windows 11
- **Go**: 1.22 以降
- **Python**: 3.10 以降（ビルドスクリプト用、標準ライブラリのみ使用）

## 2. 開発環境のセットアップ

### Go のインストール
公式サイト（https://go.dev/dl/）または Chocolatey / Winget からインストールします。

```powershell
choco install golang -y
```

インストール後、バージョンを確認します：
```powershell
go version
```

## 3. ビルド手順

### ワンクリックビルド（WASM + Go バイナリ）

リポジトリ直下の `scripts/build.bat` を実行します。

```powershell
.\scripts\build.bat
```

スクリプト内部では以下が順次実行されます：
1. `web/cmd/wasm/` を `GOOS=js GOARCH=wasm` でコンパイルし、`web/static/app.wasm` に出力
2. `web/static/` の静的リソース（HTML/CSS/WASM）を Go の `embed` パッケージで取り込み
3. `cmd/quicknetscan/` を Windows 向けにコンパイルし、`dist/QuickNetScan.exe` を生成
4. `cmd/quicknetscan-cli/` をコンパイルし、`dist/QuickNetScan_cli.exe` を生成

## 4. テスト実行

```powershell
go test ./...
```
