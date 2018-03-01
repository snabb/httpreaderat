htreaderat
==========

[![GoDoc](https://godoc.org/github.com/snabb/htreaderat?status.svg)](https://godoc.org/github.com/snabb/htreaderat)

The Go package htreaderat implements io.ReaderAt for http URLs.


Example
-------

The following example outputs a file list of remote zip archive without
downloading the whole archive:

```Go
package main

import (
	"archive/zip"
	"fmt"
	"github.com/snabb/htreaderat"
)

func main() {
	htrdr := htreaderat.New(nil, "https://dl.google.com/go/go1.10.windows-amd64.zip", nil)

	size, err := htrdr.Size()
	if err != nil {
		panic(err)
	}
	zrdr, err := zip.NewReader(htrdr, size)
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
