package httprdrat

import (
	"bytes"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"os"
)

// Should we call this "ReadFromReadAtCloser"? :)
type BackingStore interface {
	io.ReaderFrom
	io.ReaderAt
	io.Closer
}

// NewDefaultBackingStore creates a BackingStore with default settings. It
// buffers 1 MB in memory and if that is exceeded, up to 1 GB to file.
// Returned BackingStore must be Close()d if it is no longer used.
func NewDefaultBackingStore() BackingStore {
	return NewLimitedBackingStore(
		NewBackingStoreMemory(), 1024*1024, NewLimitedBackingStore(
			NewBackingStoreFile(), 1024*1024*1024, nil))
}

type BackingStoreFile struct {
	tmpfile *os.File
	size    int64
}

var _ BackingStore = (*BackingStoreFile)(nil)

func NewBackingStoreFile() *BackingStoreFile {
	return &BackingStoreFile{}
}

func (rrat *BackingStoreFile) ReadFrom(r io.Reader) (n int64, err error) {
	if rrat.tmpfile == nil {
		rrat.tmpfile, err = ioutil.TempFile("", "tmpbs")
		if err != nil {
			return 0, err
		}
	}
	n, err = io.Copy(rrat.tmpfile, r)
	rrat.size += n
	return n, err
}

func (rrat *BackingStoreFile) ReadAt(p []byte, off int64) (n int, err error) {
	return rrat.tmpfile.ReadAt(p, off)
}

func (rrat *BackingStoreFile) Size() int64 {
	return rrat.size
}

// Close must be called when the BackingStoreFile is not used any more. It
// deletes the temporary file.
func (rrat *BackingStoreFile) Close() error {
	if rrat.tmpfile == nil {
		return nil
	}
	name := rrat.tmpfile.Name()
	err := rrat.tmpfile.Close()
	err2 := os.Remove(name)
	rrat.tmpfile = nil
	rrat.size = 0

	if err == nil && err2 != nil {
		err = err2
	}
	return err
}

type BackingStoreMemory struct {
	bytes.Buffer
}

var _ BackingStore = (*BackingStoreMemory)(nil)

func NewBackingStoreMemory() *BackingStoreMemory {
	return &BackingStoreMemory{}
}

func (rrat *BackingStoreMemory) ReadAt(p []byte, off int64) (n int, err error) {
	rdr := bytes.NewReader(rrat.Bytes())
	return rdr.ReadAt(p, off)
}

func (rrat *BackingStoreMemory) Size() int64 {
	return int64(rrat.Len())
}

// Close may be called but it is not necessary.
func (rrat *BackingStoreMemory) Close() error {
	rrat.Reset()
	return nil
}

type LimitedBackingStore struct {
	bs       BackingStore
	limit    int64
	fallback BackingStore
	fellback bool
}

var _ BackingStore = (*LimitedBackingStore)(nil)

var ErrBackingStoreLimit = errors.New("backing store limit reached")

func NewLimitedBackingStore(bs BackingStore, limit int64, fallback BackingStore) *LimitedBackingStore {
	return &LimitedBackingStore{
		bs:       bs,
		limit:    limit,
		fallback: fallback,
	}
}

func (bs *LimitedBackingStore) ReadFrom(r io.Reader) (n int64, err error) {
	if bs.fellback == true {
		return bs.bs.ReadFrom(r)
	}

	lr := io.LimitReader(r, bs.limit)

	n, err = bs.bs.ReadFrom(lr)
	if n < bs.limit {
		bs.limit -= n
		return n, err
	}

	if bs.fallback == nil {
		return n, ErrBackingStoreLimit
	}

	bsrdr := io.NewSectionReader(bs.bs, 0, n)
	n, err = bs.fallback.ReadFrom(io.MultiReader(bsrdr, r))

	bs.bs.Close()

	bs.bs = bs.fallback
	bs.fellback = true

	return n, err
}

func (bs *LimitedBackingStore) ReadAt(p []byte, off int64) (n int, err error) {
	return bs.bs.ReadAt(p, off)
}

func (bs *LimitedBackingStore) Close() error {
	return bs.bs.Close()
}
