package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/klippa-app/go-pdfium/requests"
)

const usageTxt = `Usage: pdf2image txt <input.pdf> [options]

Options:
  -o <file>  出力テキストファイルパス (省略時: <pdf名>.txt)
             "-" を指定すると標準出力に出力

例:
  pdf2image txt document.pdf
  pdf2image txt document.pdf -o output.txt
  pdf2image txt document.pdf -o -
`

func runTxt(args []string) {
	fs := flag.NewFlagSet("txt", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usageTxt) }
	outputFile := fs.String("o", "", "出力テキストファイルパス")
	// "txt main.pdf -flags" / "txt -flags main.pdf" どちらの順序も許容する
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(append([]string{}, args[1:]...), args[0])
	}
	fs.Parse(args) //nolint:errcheck

	if fs.NArg() < 1 {
		fmt.Fprint(os.Stderr, usageTxt)
		os.Exit(1)
	}
	pdfPath := fs.Arg(0)

	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Fatalf("PDFファイルが見つかりません: %s", pdfPath)
	}

	// 出力先を決定
	if *outputFile == "" {
		base := filepath.Base(pdfPath)
		base = base[:len(base)-len(filepath.Ext(base))]
		*outputFile = base + ".txt"
	}

	instance, doc, close := initPDFium(pdfPath)
	defer close()

	total := pageCount(instance, doc)
	fmt.Fprintf(os.Stderr, "テキスト抽出開始: %s (%d ページ)\n", pdfPath, total)

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
		res, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    i,
				},
			},
		})
		if err != nil {
			log.Printf("[%d/%d] テキスト抽出失敗: %v", i+1, total, err)
			continue
		}

		text := strings.TrimRight(res.Text, "\r\n")

		// ページ区切り（改ページ文字）を挿入。最終ページは不要
		if i > 0 {
			fmt.Fprint(out, "\f")
		}
		fmt.Fprint(out, text)

		if *outputFile != "-" {
			fmt.Fprintf(os.Stderr, "[%d/%d] 抽出完了\n", i+1, total)
		}
	}

	// ファイル出力の場合のみ完了メッセージを表示
	if *outputFile != "-" {
		fmt.Fprintf(os.Stderr, "完了! -> %s\n", *outputFile)
	}
}
