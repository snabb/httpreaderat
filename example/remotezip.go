//
// This example outputs a single file from a remote zip file without
// downloading the whole archive. If the server does not support HTTP Range
// Requests, the whole file is downloaded to a backing store as a fallback.
//
package main

import (
	"archive/zip"
	"github.com/avvmoto/buf-readerat"
	"github.com/snabb/httprdrat"
	"io"
	"net/http"
	"os"
)

func main() {
	req, _ := http.NewRequest("GET", "https://dl.google.com/go/go1.10.windows-amd64.zip", nil)

	bs := httprdrat.NewDefaultBackingStore()
	defer bs.Close()

	htrdr, err := httprdrat.NewHTTPReaderAt(nil, req, bs)
	if err != nil {
		panic(err)
	}
	bhtrdr := bufra.NewBufReaderAt(htrdr, 1024*1024)

	zrdr, err := zip.NewReader(bhtrdr, htrdr.Size())
	if err != nil {
		panic(err)
	}
	for _, f := range zrdr.File {
		if f.Name == "go/LICENSE" {
			fr, err := f.Open()
			if err != nil {
				panic(err)
			}
			io.Copy(os.Stdout, fr)
			fr.Close()
		}
	}
}
