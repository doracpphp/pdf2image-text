package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
)

//go:embed pdfium.wasm
var pdfiumWasm []byte

const usage = `Usage: pdf2image <subcommand> [options]

Subcommands:
  image  PDFを PNG 画像に変換する
  txt    PDFに埋め込まれたテキストを抽出する
  ocr    PDFをレンダリングして NDLOCR-Lite で OCR する (*要 -tags ocr ビルド)

詳細:
  pdf2image image --help
  pdf2image txt   --help
  pdf2image ocr   --help
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "image":
		runImage(os.Args[2:])
	case "txt":
		runTxt(os.Args[2:])
	case "ocr":
		runOCR(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "不明なサブコマンド: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

// initPDFium は PDFium プールとインスタンスを初期化し、PDF を開く。
// 呼び出し元は close() を必ず呼ぶこと。
func initPDFium(pdfPath string) (pdfium.Pdfium, *responses.OpenDocument, func()) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
		WASM:     pdfiumWasm,
	})
	if err != nil {
		log.Fatalf("PDFium の初期化に失敗しました: %v", err)
	}

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		pool.Close()
		log.Fatalf("PDFium インスタンスの取得に失敗しました: %v", err)
	}

	doc, err := instance.OpenDocument(&requests.OpenDocument{FilePath: &pdfPath})
	if err != nil {
		instance.Close()
		pool.Close()
		log.Fatalf("PDF を開けませんでした: %v", err)
	}

	close := func() {
		instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}) //nolint:errcheck
		instance.Close()
		pool.Close()
	}

	return instance, doc, close
}

// pageCount は PDF のページ数を返す。
func pageCount(instance pdfium.Pdfium, doc *responses.OpenDocument) int {
	res, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		log.Fatalf("ページ数の取得に失敗しました: %v", err)
	}
	return res.PageCount
}
