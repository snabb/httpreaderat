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
		w.Header().Set("Content-Range", fmt.Sprintf(" bytes 0-0/%d", 1))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte{17})
	}))

	reader, err := ra.reader()
	ra.Nil(err)
	ra.NotNil(reader)
}

func (ra *readerAtFixture) TestRangeSupportIntialEmptyResponse() {
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

func (ra *readerAtFixture) TestRangeSupportIntialTooMuchResponse() {
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

func (ra *readerAtFixture) TestParralelFiles() {
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
