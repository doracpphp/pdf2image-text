# pdf2image

PDFファイルをPNG画像・テキスト・OCRテキストに変換するコマンドラインツールです。

## 特徴

- **シングルバイナリ** — PDFiumのWASMバイナリを実行ファイルに埋め込み済み。追加インストール不要。
- **CGO不要（標準ビルド）** — WebAssembly（Wazero）経由でPDFiumを実行するため、CGOなしでビルド可能。
- **クロスプラットフォーム** — macOS / Linux / Windows に対応。
- **高品質レンダリング** — Google ChromeでもつかわれているPDFiumエンジンを使用。
- **OCR対応（オプション）** — NDLOCR-Lite による日本語OCRをオプションビルドで追加可能。

## 使い方

```
pdf2image <subcommand> [options]

Subcommands:
  image  PDFをPNG画像に変換する
  txt    PDFに埋め込まれたテキストを抽出する
  ocr    PDFをレンダリングして NDLOCR-Lite で OCR する  (*要 -tags ocr ビルド)
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

### txt — PDF → テキスト（テキスト埋め込みPDF向け）

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
スキャンされたPDF（画像のみ）はテキスト抽出できません。その場合は `ocr` サブコマンドを使ってください。

### ocr — PDF → テキスト（スキャンPDF・OCR向け）

> **注意:** `ocr` サブコマンドは `-tags ocr` でビルドしたバイナリが必要です。
> ビルド方法は「[OCR機能のビルド](#ocr機能のビルド)」を参照してください。

```
pdf2image ocr <input.pdf> [options]

Options:
  -det-model  <path>   検出モデルファイル (ONNX)          [必須]
  -det-config <path>   検出設定ファイル (YAML)            [必須]
  -rec-model  <path>   認識モデルファイル (ONNX)          [必須]
  -rec-config <path>   認識設定ファイル (YAML)            [必須]
  -ort-lib    <path>   ONNX Runtime 共有ライブラリのパス
                       (省略時: $ONNXRUNTIME_LIB または OS デフォルト)
  -threshold  <float>  検出信頼度の閾値 (省略時: 0.25)
  -dpi        <int>    レンダリング解像度、高いほど精度向上 (省略時: 300)
  -o          <file>   出力テキストファイル (省略時: <pdf名>_ocr.txt)
                       "-" を指定すると標準出力に出力
```

```bash
pdf2image ocr scan.pdf \
  -det-model ndl_det.onnx -det-config ndl_det.yaml \
  -rec-model ndl_rec.onnx -rec-config ndl_rec.yaml

# DPI を上げて精度向上
pdf2image ocr scan.pdf \
  -det-model ndl_det.onnx -det-config ndl_det.yaml \
  -rec-model ndl_rec.onnx -rec-config ndl_rec.yaml \
  -dpi 400 -o result.txt
```

処理フロー: PDFium でページをレンダリング → NDLOCR-Lite で行検出 → 行ごとにテキスト認識
ページ間の区切りには改ページ文字（`\f`）が挿入されます。

## ビルド

### 前提条件

- Go 1.25 以上

### 標準ビルド（image / txt サブコマンド）

```bash
# リポジトリをクローン
git clone <repo-url>
cd pdf2image

# 依存パッケージを取得
go mod tidy

# pdfium.wasm をモジュールキャッシュからコピー
cp $(go env GOPATH)/pkg/mod/github.com/klippa-app/go-pdfium@v1.18.0/webassembly/pdfium.wasm .

# ビルド（CGO不要）
go build -o pdf2image .
```

### クロスコンパイル（Windows向け）

CGOが不要なため、macOS / Linux から Windows バイナリを簡単に生成できます。

```bash
GOOS=windows GOARCH=amd64 go build -o pdf2image.exe .
```

### OCR機能のビルド

`ocr` サブコマンドは [NDLOCR-Lite](https://github.com/mattn/go-ndlocr) と [ONNX Runtime](https://onnxruntime.ai/) を使用するため、CGOが必要です。

#### 1. ONNX Runtime をインストール

公式サイト（https://github.com/microsoft/onnxruntime/releases）から共有ライブラリをダウンロードしてインストールします。

```bash
# macOS の例
brew install onnxruntime

# Linux の例（/usr/local/lib に配置）
wget https://github.com/microsoft/onnxruntime/releases/download/v1.21.0/onnxruntime-linux-x64-1.21.0.tgz
tar xf onnxruntime-linux-x64-1.21.0.tgz
cp onnxruntime-linux-x64-1.21.0/lib/libonnxruntime.so.1.21.0 /usr/local/lib/libonnxruntime.so
```

#### 2. NDLOCR-Lite モデルファイルを用意

[NDLOCR-Lite](https://github.com/ndl-lab/ndlocr_cli) から以下の4ファイルをダウンロードします。

| ファイル | 用途 |
|---------|------|
| 検出モデル (`.onnx`) | テキスト行の領域検出 |
| 検出設定 (`.yaml`) | 検出モデルのクラス定義 |
| 認識モデル (`.onnx`) | テキスト行の文字認識 |
| 認識設定 (`.yaml`) | 認識モデルの文字リスト定義 |

#### 3. OCR 対応バイナリをビルド

```bash
go build -tags ocr -o pdf2image .
```

## `txt` と `ocr` の使い分け

| | `txt` | `ocr` |
|---|---|---|
| 対象 | テキスト埋め込みPDF | スキャンPDF・画像PDF |
| 速度 | 高速 | 低速（ニューラルネット推論） |
| 精度 | PDF内のテキストそのまま | OCRエンジン依存 |
| 外部依存 | なし | ONNX Runtime + モデルファイル |
| ビルドタグ | 不要 | `-tags ocr` |

## 技術詳細

| 項目 | 内容 |
|------|------|
| PDFエンジン | [PDFium](https://pdfium.googlesource.com/pdfium/) |
| Goバインディング | [go-pdfium](https://github.com/klippa-app/go-pdfium) v1.18.0 |
| WASMランタイム | [Wazero](https://github.com/tetratelabs/wazero) v1.11.0 |
| WASMバイナリ | `pdfium.wasm`（3.8MB、バイナリに埋め込み済み） |
| OCRエンジン | [NDLOCR-Lite](https://github.com/mattn/go-ndlocr)（`-tags ocr` 時のみ） |
| 推論ランタイム | [ONNX Runtime](https://onnxruntime.ai/)（`-tags ocr` 時のみ） |

PDFiumのWASMバイナリは `//go:embed pdfium.wasm` によってビルド時に実行ファイルへ埋め込まれます。そのため、標準ビルドのバイナリは単体で動作します。
