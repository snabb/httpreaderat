//
// This example outputs a single file from a remote zip file without
// downloading the whole archive. If the server does not support HTTP Range
// Requests, the whole file is downloaded to a backing store as a fallback.
//
package main

import (
	"archive/zip"
	"fmt"
	"github.com/avvmoto/buf-readerat"
	"github.com/snabb/httpreaderat"
	"io"
	"net/http"
	"os"
)

func catZipFile(url, fileName string) (err error) {
	// create http.Request
	req, _ := http.NewRequest("GET", url, nil)

	// a backing store in case the server does not support range requests
	bs := httpreaderat.NewDefaultStore()
	defer bs.Close()

	// make a HTTPReaderAt client
	htrdr, err := httpreaderat.New(nil, req, bs)
	if err != nil {
		return err
	}

	// make it buffered
	bhtrdr := bufra.NewBufReaderAt(htrdr, 1024*1024)

	// make a ZIP file reader
	zrdr, err := zip.NewReader(bhtrdr, htrdr.Size())
	if err != nil {
		return err
	}
	// go through the ZIP file contents until desired file is found
	for _, f := range zrdr.File {
		if f.Name == fileName {
			fr, err := f.Open()
			if err != nil {
				return err
			}
			io.Copy(os.Stdout, fr)
			fr.Close()
			return nil
		}
	}
	return os.ErrNotExist
}

func main() {
	err := catZipFile(
		"https://dl.google.com/go/go1.12.windows-amd64.zip",
		"go/LICENSE")

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
