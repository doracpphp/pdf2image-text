//go:build ocr

package main

import (
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ndlocr "github.com/mattn/go-ndlocr"
	ort "github.com/yalue/onnxruntime_go"

	"github.com/klippa-app/go-pdfium/requests"
)

const usageOCR = `Usage: pdf2image ocr <input.pdf> [options]

PDFの各ページを画像にレンダリングし、NDLOCR-Lite で OCR してテキストを出力します。
スキャン済みPDFや画像PDF に有効です。

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

例:
  pdf2image ocr scan.pdf \
    -det-model ndl_det.onnx -det-config ndl_det.yaml \
    -rec-model ndl_rec.onnx -rec-config ndl_rec.yaml
`

// defaultORTLib はプラットフォームごとのデフォルト ONNX Runtime ライブラリパスを返す
func defaultORTLib() string {
	if v := os.Getenv("ONNXRUNTIME_LIB"); v != "" {
		return v
	}
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "darwin":
		return "/usr/local/lib/libonnxruntime.dylib"
	default:
		return "/usr/local/lib/libonnxruntime.so"
	}
}

func runOCR(args []string) {
	fs := flag.NewFlagSet("ocr", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usageOCR) }

	detModel   := fs.String("det-model", "", "検出モデル (ONNX)")
	detConf    := fs.String("det-config", "", "検出設定 (YAML)")
	recModel   := fs.String("rec-model", "", "認識モデル (ONNX)")
	recConf    := fs.String("rec-config", "", "認識設定 (YAML)")
	ortLib     := fs.String("ort-lib", defaultORTLib(), "ONNX Runtime 共有ライブラリ")
	thresh     := fs.Float64("threshold", 0.25, "検出信頼度閾値")
	dpi        := fs.Int("dpi", 300, "レンダリング解像度")
	outputFile := fs.String("o", "", "出力テキストファイル")
	// "ocr main.pdf -flags" / "ocr -flags main.pdf" どちらの順序も許容する
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	fs.Parse(args) //nolint:errcheck

	if fs.NArg() < 1 {
		fmt.Fprint(os.Stderr, usageOCR)
		os.Exit(1)
	}
	pdfPath := fs.Arg(0)

	if *detModel == "" || *detConf == "" || *recModel == "" || *recConf == "" {
		fmt.Fprintln(os.Stderr, "エラー: -det-model, -det-config, -rec-model, -rec-config は必須です")
		fmt.Fprint(os.Stderr, usageOCR)
		os.Exit(1)
	}

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Fatalf("PDFファイルが見つかりません: %s", pdfPath)
	}

	// 出力先を決定
	if *outputFile == "" {
		base := filepath.Base(pdfPath)
		base = base[:len(base)-len(filepath.Ext(base))]
		*outputFile = base + "_ocr.txt"
	}

	// ONNX Runtime を初期化
	ort.SetSharedLibraryPath(*ortLib)
	if err := ort.InitializeEnvironment(); err != nil {
		log.Fatalf("ONNX Runtime の初期化に失敗しました: %v\nライブラリパスを -ort-lib で指定してください", err)
	}
	defer ort.DestroyEnvironment()

	det, err := ndlocr.NewDetector(*detModel, *detConf)
	if err != nil {
		log.Fatalf("Detector の読み込みに失敗しました: %v", err)
	}
	defer det.Close()

	rec, err := ndlocr.NewRecognizer(*recModel, *recConf)
	if err != nil {
		log.Fatalf("Recognizer の読み込みに失敗しました: %v", err)
	}
	defer rec.Close()

	// PDFium を初期化
	instance, doc, closePDF := initPDFium(pdfPath)
	defer closePDF()

	total := pageCount(instance, doc)
	fmt.Fprintf(os.Stderr, "OCR開始: %s (%d ページ, %d DPI)\n", pdfPath, total, *dpi)

	// 出力先を開く
	var out *os.File
	if *outputFile == "-" {
		out = os.Stdout
	} else {
		f, err := os.Create(*outputFile)
		if err != nil {
			log.Fatalf("出力ファイルの作成に失敗しました: %v", err)
		}
		defer f.Close()
		out = f
	}

	for i := 0; i < total; i++ {
		// PDFium でページを画像にレンダリング
		rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
			DPI: *dpi,
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    i,
				},
			},
		})
		if err != nil {
			log.Printf("[%d/%d] レンダリング失敗: %v", i+1, total, err)
			continue
		}

		// NDLOCR-Lite で OCR
		lines, err := ocrPage(det, rec, rendered.Result.Image, *thresh)
		rendered.Cleanup()

		if err != nil {
			log.Printf("[%d/%d] OCR失敗: %v", i+1, total, err)
			continue
		}

		// ページ区切り（最初のページ以外）
		if i > 0 {
			fmt.Fprint(out, "\f")
		}
		fmt.Fprint(out, strings.Join(lines, "\n"))

		fmt.Fprintf(os.Stderr, "[%d/%d] %d 行検出\n", i+1, total, len(lines))
	}

	if *outputFile != "-" {
		fmt.Fprintf(os.Stderr, "完了! -> %s\n", *outputFile)
	}
}

// ocrPage は 1 ページ分の画像から NDLOCR-Lite でテキスト行を抽出して返す
func ocrPage(det *ndlocr.Detector, rec *ndlocr.Recognizer, img image.Image, thresh float64) ([]string, error) {
	detections, err := det.Detect(img)
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, d := range detections {
		if float64(d.Confidence) < thresh {
			continue
		}
		// IsLine は class 名が "line_" で始まるものだけを通す
		if !det.IsLine(d.ClassIndex) {
			continue
		}

		rect := image.Rect(d.Box[0], d.Box[1], d.Box[2], d.Box[3]).Intersect(img.Bounds())
		if rect.Empty() {
			continue
		}

		// SubImage は *image.RGBA が実装しているが、interface 経由でアクセスする
		type subImager interface {
			SubImage(r image.Rectangle) image.Image
		}
		si, ok := img.(subImager)
		if !ok {
			continue
		}

		text, err := rec.Recognize(si.SubImage(rect))
		if err != nil || text == "" {
			continue
		}
		lines = append(lines, text)
	}
	return lines, nil
}
