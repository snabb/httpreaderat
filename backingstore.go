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

func (bs *BackingStoreFile) ReadFrom(r io.Reader) (n int64, err error) {
	if bs.tmpfile == nil {
		bs.tmpfile, err = ioutil.TempFile("", "tmpbs")
		if err != nil {
			return 0, err
		}
	}
	n, err = io.Copy(bs.tmpfile, r)
	bs.size += n
	return n, err
}

func (bs *BackingStoreFile) ReadAt(p []byte, off int64) (n int, err error) {
	return bs.tmpfile.ReadAt(p, off)
}

func (bs *BackingStoreFile) Size() int64 {
	return bs.size
}

// Close must be called when the BackingStoreFile is not used any more. It
// deletes the temporary file.
func (bs *BackingStoreFile) Close() error {
	if bs.tmpfile == nil {
		return nil
	}
	name := bs.tmpfile.Name()
	err := bs.tmpfile.Close()
	err2 := os.Remove(name)
	bs.tmpfile = nil
	bs.size = 0

	if err == nil && err2 != nil {
		err = err2
	}
	return err
}

type BackingStoreMemory struct {
	buf bytes.Buffer
}

var _ BackingStore = (*BackingStoreMemory)(nil)

func NewBackingStoreMemory() *BackingStoreMemory {
	return &BackingStoreMemory{}
}

func (bs *BackingStoreMemory) ReadFrom(r io.Reader) (n int64, err error) {
	return bs.buf.ReadFrom(r)
}

func (bs *BackingStoreMemory) ReadAt(p []byte, off int64) (n int, err error) {
	rdr := bytes.NewReader(bs.buf.Bytes())
	return rdr.ReadAt(p, off)
}

func (bs *BackingStoreMemory) Size() int64 {
	return int64(bs.buf.Len())
}

// Close may be called but it is not necessary.
func (bs *BackingStoreMemory) Close() error {
	bs.buf.Reset()
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
