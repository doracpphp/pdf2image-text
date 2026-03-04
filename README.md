# pdf2image

PDFファイルをPNG画像またはテキストに変換するコマンドラインツールです。

## 特徴

- **シングルバイナリ** — PDFiumのWASMバイナリを実行ファイルに埋め込み済み。追加インストール不要。
- **CGO不要** — WebAssembly（Wazero）経由でPDFiumを実行するため、CGOなしでビルド可能。
- **クロスプラットフォーム** — macOS / Linux / Windows に対応。
- **高品質レンダリング** — Google ChromeでもつかわれているPDFiumエンジンを使用。

## 使い方

```
pdf2image <subcommand> [options]

Subcommands:
  image  PDFをPNG画像に変換する
  txt    PDFをテキストファイルに変換する
```

### image — PDF → PNG

```
pdf2image image <input.pdf> [options]

Options:
  -o <dir>   PNG出力先ディレクトリ (省略時: カレントディレクトリ)
  -dpi <n>   解像度 (省略時: 150)
```

```bash
# カレントディレクトリに出力（150 DPI）
pdf2image image document.pdf

# output/ ディレクトリに 300 DPI で出力
pdf2image image document.pdf -o ./output -dpi 300
```

出力ファイル名は `<PDFファイル名>_0001.png`、`<PDFファイル名>_0002.png` ... の形式になります。

### txt — PDF → テキスト

```
pdf2image txt <input.pdf> [options]

Options:
  -o <file>  出力テキストファイルパス (省略時: <pdf名>.txt)
             "-" を指定すると標準出力に出力
```

```bash
# <pdf名>.txt に出力
pdf2image txt document.pdf

# 出力先を指定
pdf2image txt document.pdf -o output.txt

# 標準出力に出力（パイプ利用可）
pdf2image txt document.pdf -o -
```

ページ間の区切りには改ページ文字（`\f`）が挿入されます。
スキャンされたPDF（画像のみ）はテキスト抽出できません。

## ビルド

### 前提条件

- Go 1.25 以上

### 手順

```bash
# リポジトリをクローン
git clone <repo-url>
cd pdf2image

# 依存パッケージを取得
go mod tidy

# pdfium.wasm をモジュールキャッシュからコピー
cp $(go env GOPATH)/pkg/mod/github.com/klippa-app/go-pdfium@v1.18.0/webassembly/pdfium.wasm .

# ビルド
go build -o pdf2image .
```

### クロスコンパイル（Windows向け）

CGOが不要なため、macOS / Linux から Windows バイナリを簡単に生成できます。

```bash
GOOS=windows GOARCH=amd64 go build -o pdf2image.exe .
```

## 技術詳細

| 項目 | 内容 |
|------|------|
| PDFエンジン | [PDFium](https://pdfium.googlesource.com/pdfium/) |
| Goバインディング | [go-pdfium](https://github.com/klippa-app/go-pdfium) v1.18.0 |
| WASMランタイム | [Wazero](https://github.com/tetratelabs/wazero) v1.11.0 |
| WASMバイナリ | `pdfium.wasm`（3.8MB、バイナリに埋め込み済み） |

PDFiumのWASMバイナリは `//go:embed pdfium.wasm` によってビルド時に実行ファイルへ埋め込まれます。そのため、生成されたバイナリ単体で動作します。
