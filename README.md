htreaderat
==========

[![GoDoc](https://godoc.org/github.com/snabb/htreaderat?status.svg)](https://godoc.org/github.com/snabb/htreaderat)

The Go package htreaderat implements io.ReaderAt for HTTP requests.


Example
-------

The following example outputs a file list of remote zip archive without
downloading the whole archive:

```Go
package main

import (
	"archive/zip"
	"fmt"
	"github.com/avvmoto/buf-readerat"
	"github.com/snabb/htreaderat"
	"net/http"
)

func main() {
	req, _ := http.NewRequest("GET", "https://dl.google.com/go/go1.10.windows-amd64.zip", nil)
	htrdr, _ := htreaderat.New(nil, req)
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
```


License
-------

MIT
