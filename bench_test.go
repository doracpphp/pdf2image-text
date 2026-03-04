package main

import (
	"image/png"
	"io"
	"os"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

const testPDF = "main.pdf"

// BenchmarkRenderPageInDPI は 1 ページのレンダリング速度を DPI 別に計測する
func BenchmarkRenderPageInDPI(b *testing.B) {
	dpis := []struct {
		name string
		dpi  int
	}{
		{"72dpi", 72},
		{"150dpi", 150},
		{"300dpi", 300},
	}

	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
		WASM:     pdfiumWasm,
	})
	if err != nil {
		b.Fatalf("pool init: %v", err)
	}
	defer pool.Close()

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		b.Fatalf("get instance: %v", err)
	}
	defer instance.Close()

	pdfPath := testPDF
	doc, err := instance.OpenDocument(&requests.OpenDocument{FilePath: &pdfPath})
	if err != nil {
		b.Fatalf("open document: %v", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}) //nolint:errcheck

	for _, tc := range dpis {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
					DPI: tc.dpi,
					Page: requests.Page{
						ByIndex: &requests.PageByIndex{
							Document: doc.Document,
							Index:    0,
						},
					},
				})
				if err != nil {
					b.Fatalf("render: %v", err)
				}
				rendered.Cleanup()
			}
		})
	}
}

// BenchmarkFullConversion は全ページをレンダリングして PNG エンコードするまでを計測する
func BenchmarkFullConversion(b *testing.B) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
		WASM:     pdfiumWasm,
	})
	if err != nil {
		b.Fatalf("pool init: %v", err)
	}
	defer pool.Close()

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		b.Fatalf("get instance: %v", err)
	}
	defer instance.Close()

	pdfPath := testPDF
	doc, err := instance.OpenDocument(&requests.OpenDocument{FilePath: &pdfPath})
	if err != nil {
		b.Fatalf("open document: %v", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}) //nolint:errcheck

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		b.Fatalf("get page count: %v", err)
	}

	total := pageCount.PageCount
	b.ReportMetric(float64(total), "pages/run")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for p := 0; p < total; p++ {
			rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
				DPI: 150,
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{
						Document: doc.Document,
						Index:    p,
					},
				},
			})
			if err != nil {
				b.Fatalf("render page %d: %v", p, err)
			}
			if err := png.Encode(io.Discard, rendered.Result.Image); err != nil {
				b.Fatalf("encode page %d: %v", p, err)
			}
			rendered.Cleanup()
		}
	}

	b.ReportMetric(float64(b.N*total)/b.Elapsed().Seconds(), "pages/sec")
}

// BenchmarkRenderVsEncode はレンダリングと PNG エンコードの所要時間を分離して計測する
func BenchmarkRenderVsEncode(b *testing.B) {
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
		WASM:     pdfiumWasm,
	})
	if err != nil {
		b.Fatalf("pool init: %v", err)
	}
	defer pool.Close()

	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		b.Fatalf("get instance: %v", err)
	}
	defer instance.Close()

	pdfPath := testPDF
	doc, err := instance.OpenDocument(&requests.OpenDocument{FilePath: &pdfPath})
	if err != nil {
		b.Fatalf("open document: %v", err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document}) //nolint:errcheck

	b.Run("RenderOnly", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
				DPI: 150,
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0},
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			rendered.Cleanup()
		}
	})

	b.Run("RenderAndEncode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
				DPI: 150,
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0},
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			if err := png.Encode(io.Discard, rendered.Result.Image); err != nil {
				b.Fatal(err)
			}
			rendered.Cleanup()
		}
	})

	b.Run("WriteToFile", func(b *testing.B) {
		f, err := os.CreateTemp("", "bench-*.png")
		if err != nil {
			b.Fatal(err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		for i := 0; i < b.N; i++ {
			rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
				DPI: 150,
				Page: requests.Page{
					ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0},
				},
			})
			if err != nil {
				b.Fatal(err)
			}
			f.Seek(0, io.SeekStart) //nolint:errcheck
			if err := png.Encode(f, rendered.Result.Image); err != nil {
				b.Fatal(err)
			}
			rendered.Cleanup()
		}
	})
}
