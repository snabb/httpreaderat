httpreaderat
============

[![GoDoc](https://godoc.org/github.com/snabb/httpreaderat?status.svg)](https://godoc.org/github.com/snabb/httpreaderat)

Go package httpreaderat implements io.ReaderAt that makes HTTP Range Requests.

It can be used for example with "archive/zip" package in Go standard
library. Together they can be used to access remote (HTTP accessible)
ZIP archives without needing to download the whole archive file.

HTTP Range Requests (see [RFC 7233](https://tools.ietf.org/html/rfc7233))
are used to retrieve the requested byte range. There is an optional fallback
mechanism which can be used to download the whole file and buffer it locally
if the server does not support Range Requests.

When using this package with "archive/zip", it is a good idea to also use
"[github.com/avvmoto/buf-readerat](https://github.com/avvmoto/buf-readerat)"
which implements a buffered io.ReaderAt "proxy". It reduces the amount of
small HTTP requests significantly. 1 MB is a good buffer size to use. See
the example below for details.

If you need io.ReadSeeker (with Read() and Seek() methods) to be used for
example with "archive/tar", you can wrap HTTPReaderAt with io.SectionReader.


Example
-------

The following example outputs a file list of a remote zip archive without
downloading the whole archive:

```Go
package main

import (
	"archive/zip"
	"fmt"
	"github.com/avvmoto/buf-readerat"
	"github.com/snabb/httpreaderat"
	"net/http"
)

func main() {
	req, _ := http.NewRequest("GET", "https://dl.google.com/go/go1.10.windows-amd64.zip", nil)

	htrdr, err := httpreaderat.New(nil, req, nil)
	if err != nil {
		panic(err)
	}
	bhtrdr := bufra.NewBufReaderAt(htrdr, 1024*1024)

	zrdr, err := zip.NewReader(bhtrdr, htrdr.Size())
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
