//go:build !ocr

package main

import (
	"fmt"
	"os"
)

func runOCR(_ []string) {
	fmt.Fprintln(os.Stderr, "OCR機能は標準ビルドに含まれていません。")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "OCR対応バイナリをビルドするには:")
	fmt.Fprintln(os.Stderr, "  go build -tags ocr -o pdf2image .")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "詳細は README.md の「OCR機能のビルド」を参照してください。")
	os.Exit(1)
}
