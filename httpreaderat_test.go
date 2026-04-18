package httpreaderat_test

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/snabb/httpreaderat"
	"github.com/stretchr/testify/suite"
)

type readerAtFixture struct {
	suite.Suite
	server *httptest.Server
}

func (ra *readerAtFixture) AfterTest(suiteName, testName string) {
	if ra.server != nil {
		ra.server.Close()
	}
}

func (ra *readerAtFixture) reader() (*httpreaderat.HTTPReaderAt, error) {
	req, err := http.NewRequest("GET", ra.server.URL+"/file.zip", nil)
	ra.Nil(err)
	return httpreaderat.New(nil, req, nil)
}

func TestReaderAtFixture(t *testing.T) {
	suite.Run(t, new(readerAtFixture))
}

func (ra *readerAtFixture) TestRangeNotSupported() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ra.Equal(r.Method, "GET")
		ra.Equal(r.URL.String(), "/file.zip")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":{"data": [1,2,3]}}`))
	}))

	reader, err := ra.reader()
	ra.EqualError(err, "server does not support range requests")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestNonGetRequestMethod() {
	req, err := http.NewRequest("POST", "http://not-valid.url/file.zip", nil)
	ra.Nil(err)
	reader, err := httpreaderat.New(nil, req, nil)
	ra.EqualError(err, "invalid HTTP method")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestRangeSupportInitial() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rnge := r.Header.Get("Range")
		ra.Equal(rnge, "bytes=0-0")
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		w.Header().Set("Content-Range", fmt.Sprintf(" bytes 0-0/%d", 1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{17})
	}))

	reader, err := ra.reader()
	ra.Nil(err)
	ra.NotNil(reader)
	ra.Equal("application/zip", reader.ContentType())
	ra.Equal("Wed, 21 Oct 2015 07:28:00 GMT", reader.LastModified())
	ra.Equal(int64(1), reader.Size())
}

func (ra *readerAtFixture) TestRangeSupportInitialEmptyResponse() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rnge := r.Header.Get("Range")
		ra.Equal(rnge, "bytes=0-0")
		w.Header().Set("Content-Range", fmt.Sprintf(" bytes 0-0/%d", 1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{})
	}))

	reader, err := ra.reader()
	ra.EqualError(err, "content-length mismatch in http response")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestRangeSupportInitialTooMuchResponse() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rnge := r.Header.Get("Range")
		ra.Equal(rnge, "bytes=0-0")
		w.Header().Set("Content-Range", fmt.Sprintf(" bytes 0-0/%d", 1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{17, 18, 19}) // should be 1
	}))

	reader, err := ra.reader()
	ra.EqualError(err, "content-length mismatch in http response")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestFile() {
	ra.server = httptest.NewServer(http.FileServer(http.Dir("./fixtures")))

	reader, err := ra.reader()
	ra.Nil(err)
	ra.NotNil(reader)

	zfile, err := zip.NewReader(reader, reader.Size())
	ra.NoError(err)
	ra.Len(zfile.File, 4)

	txtfile, err := zfile.Open("test2")
	ra.NoError(err)
	defer txtfile.Close()

	content, err := io.ReadAll(txtfile)
	ra.NoError(err)

	ra.Equal(http.DetectContentType(content), "text/plain; charset=utf-8")
}

func (ra *readerAtFixture) TestParallelFiles() {
	ra.server = httptest.NewServer(http.FileServer(http.Dir("./fixtures")))

	reader, err := ra.reader()
	ra.Nil(err)
	ra.NotNil(reader)

	zfile, err := zip.NewReader(reader, reader.Size())
	ra.NoError(err)
	ra.Len(zfile.File, 4)

	var wg sync.WaitGroup

	testread := func(f *zip.File) {
		defer wg.Done()
		file, err := f.Open()
		ra.NoError(err)
		defer file.Close()

		content, err := io.ReadAll(file)
		ra.NoError(err)
		ra.NotEmpty(content, "name", f.Name)
	}

	for _, f := range zfile.File {
		wg.Add(1)
		go testread(f)
	}

	wg.Wait()

}

func (ra *readerAtFixture) TestReadAtEOFClamp() {
	content := []byte("hello")
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Range") {
		case "bytes=0-0":
			w.Header().Set("Content-Range", "bytes 0-0/5")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(content[:1])
		case "bytes=3-4":
			w.Header().Set("Content-Range", "bytes 3-4/5")
			w.WriteHeader(http.StatusPartialContent)
			w.Write(content[3:])
		default:
			ra.FailNow("unexpected range", r.Header.Get("Range"))
		}
	}))

	reader, err := ra.reader()
	ra.NoError(err)

	buf := make([]byte, 4)
	n, err := reader.ReadAt(buf, 3)
	ra.Equal(2, n)
	ra.ErrorIs(err, io.EOF)
	ra.Equal("lo", string(buf[:n]))
}

func (ra *readerAtFixture) TestReadAtPastEOF() {
	content := []byte("hello")
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ra.Equal("bytes=0-0", r.Header.Get("Range"))
		w.Header().Set("Content-Range", "bytes 0-0/5")
		w.WriteHeader(http.StatusPartialContent)
		w.Write(content[:1])
	}))

	reader, err := ra.reader()
	ra.NoError(err)

	n, err := reader.ReadAt(make([]byte, 1), int64(len(content)))
	ra.Equal(0, n)
	ra.ErrorIs(err, io.EOF)
}

func (ra *readerAtFixture) TestValidationFailure() {
	etag := `"v1"`
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Range") {
		case "bytes=0-0":
			w.Header().Set("Content-Range", "bytes 0-0/5")
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte("h"))
		case "bytes=1-1":
			w.Header().Set("Content-Range", "bytes 1-1/5")
			w.Header().Set("ETag", `"v2"`)
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte("e"))
		default:
			ra.FailNow("unexpected range", r.Header.Get("Range"))
		}
	}))

	reader, err := ra.reader()
	ra.NoError(err)

	n, err := reader.ReadAt(make([]byte, 1), 1)
	ra.Equal(0, n)
	ra.ErrorIs(err, httpreaderat.ErrValidationFailed)
}

func (ra *readerAtFixture) TestMissingContentRange() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("h"))
	}))

	reader, err := ra.reader()
	ra.EqualError(err, "no content-range header in partial response")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestMalformedContentRange() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "banana")
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("h"))
	}))

	reader, err := ra.reader()
	ra.EqualError(err, "http request: unsupported unit")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestDifferentRangeThanRequested() {
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Range", "bytes 1-1/5")
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte("h"))
	}))

	reader, err := ra.reader()
	ra.EqualError(err, "received different range than requested (req=0-0, resp=1-1)")
	ra.Nil(reader)
}

func (ra *readerAtFixture) TestHTTPRequestFailure() {
	req, err := http.NewRequest("GET", "http://127.0.0.1:1/file.zip", nil)
	ra.NoError(err)

	reader, err := httpreaderat.New(&http.Client{}, req, nil)
	ra.Nil(reader)
	ra.Error(err)
}

func (ra *readerAtFixture) TestServerStopsSupportingRangeRequests() {
	requests := 0
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Content-Range", "bytes 0-0/5")
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte("h"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	req, err := http.NewRequest("GET", ra.server.URL+"/file.zip", nil)
	ra.NoError(err)

	reader, err := httpreaderat.New(nil, req, httpreaderat.NewStoreMemory())
	ra.NoError(err)

	n, err := reader.ReadAt(make([]byte, 1), 1)
	ra.Equal(0, n)
	ra.EqualError(err, "server suddenly stopped supporting range requests")
}

func (ra *readerAtFixture) TestFallbackStoreWhenRangeNotSupported() {
	content := []byte("hello")
	ra.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Last-Modified", "Wed, 21 Oct 2015 07:28:00 GMT")
		w.WriteHeader(http.StatusOK)
		w.Write(content)
	}))

	req, err := http.NewRequest("GET", ra.server.URL+"/file.zip", nil)
	ra.NoError(err)

	reader, err := httpreaderat.New(nil, req, httpreaderat.NewStoreMemory())
	ra.NoError(err)
	ra.Equal("text/plain", reader.ContentType())
	ra.Equal("Wed, 21 Oct 2015 07:28:00 GMT", reader.LastModified())
	ra.Equal(int64(len(content)), reader.Size())

	buf := make([]byte, len(content))
	n, err := reader.ReadAt(buf, 0)
	ra.NoError(err)
	ra.Equal(len(content), n)
	ra.Equal(content, buf)
}
