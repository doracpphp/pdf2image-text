package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/klippa-app/go-pdfium/requests"
)

const usageImage = `Usage: pdf2image image <input.pdf> [options]

Options:
  -o <dir>   PNG出力先ディレクトリ (省略時: カレントディレクトリ)
  -dpi <n>   解像度 (省略時: 150)

例:
  pdf2image image document.pdf
  pdf2image image document.pdf -o ./output -dpi 300
`

func runImage(args []string) {
	fs := flag.NewFlagSet("image", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usageImage) }
	outputDir := fs.String("o", ".", "PNG出力先ディレクトリ")
	dpi := fs.Int("dpi", 150, "解像度")
	// "image main.pdf -flags" / "image -flags main.pdf" どちらの順序も許容する
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	fs.Parse(args) //nolint:errcheck

	if fs.NArg() < 1 {
		fmt.Fprint(os.Stderr, usageImage)
		os.Exit(1)
	}
	pdfPath := fs.Arg(0)

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Fatalf("PDFファイルが見つかりません: %s", pdfPath)
	}
	if *dpi <= 0 {
		log.Fatalf("DPI は正の整数を指定してください")
	}

	instance, doc, close := initPDFium(pdfPath)
	defer close()

	total := pageCount(instance, doc)
	fmt.Printf("変換開始: %s (%d ページ, %d DPI)\n", pdfPath, total, *dpi)

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		log.Fatalf("出力ディレクトリの作成に失敗しました: %v", err)
	}

	base := filepath.Base(pdfPath)
	base = base[:len(base)-len(filepath.Ext(base))]

	for i := 0; i < total; i++ {
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
		defer rendered.Cleanup()

		outPath := filepath.Join(*outputDir, fmt.Sprintf("%s_%04d.png", base, i+1))
		f, err := os.Create(outPath)
		if err != nil {
			log.Printf("[%d/%d] ファイル作成失敗 %s: %v", i+1, total, outPath, err)
			continue
		}
		if err := png.Encode(f, rendered.Result.Image); err != nil {
			f.Close()
			log.Printf("[%d/%d] PNGエンコード失敗: %v", i+1, total, err)
			continue
		}
		f.Close()
		fmt.Printf("[%d/%d] %s\n", i+1, total, outPath)
	}

	fmt.Println("完了!")
}
