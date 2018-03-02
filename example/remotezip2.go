//
// This example outputs a file list of remote zip archive without
// downloading the whole archive.
//
package main

import (
	"archive/zip"
	"fmt"
	"github.com/avvmoto/buf-readerat"
	"github.com/snabb/htreaderat"
)

func main() {
	htrdr := htreaderat.New(nil, "https://dl.google.com/go/go1.10.windows-amd64.zip", nil)
	bhtrdr := bufra.NewBufReaderAt(htrdr, 1024*1024)

	size, err := htrdr.Size()
	if err != nil {
		panic(err)
	}
	zrdr, err := zip.NewReader(bhtrdr, size)
	if err != nil {
		panic(err)
	}
	for _, f := range zrdr.File {
		fmt.Println(f.Name)
	}
}
