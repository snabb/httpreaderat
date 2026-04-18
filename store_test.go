package httpreaderat

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
)

func TestStoreMemory(t *testing.T) {
	s := NewStoreMemory()

	if n, err := s.ReadAt(make([]byte, 1), 0); n != 0 || err != nil {
		t.Fatalf("empty ReadAt = (%d, %v), want (0, nil)", n, err)
	}

	n, err := s.ReadFrom(bytes.NewBufferString("hello world"))
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if n != 11 {
		t.Fatalf("ReadFrom() n = %d, want 11", n)
	}
	if got := s.Size(); got != 11 {
		t.Fatalf("Size() = %d, want 11", got)
	}

	buf := make([]byte, 5)
	nn, err := s.ReadAt(buf, 6)
	if err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if nn != 5 || string(buf) != "world" {
		t.Fatalf("ReadAt() = (%d, %q), want (5, %q)", nn, string(buf), "world")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if got := s.Size(); got != 0 {
		t.Fatalf("Size() after Close = %d, want 0", got)
	}
}

func TestStoreFile(t *testing.T) {
	s := NewStoreFile()

	if n, err := s.ReadAt(make([]byte, 1), 0); n != 0 || err != nil {
		t.Fatalf("empty ReadAt = (%d, %v), want (0, nil)", n, err)
	}

	n, err := s.ReadFrom(bytes.NewBufferString("abcdef"))
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if n != 6 {
		t.Fatalf("ReadFrom() n = %d, want 6", n)
	}
	if got := s.Size(); got != 6 {
		t.Fatalf("Size() = %d, want 6", got)
	}
	if s.tmpfile == nil {
		t.Fatal("tmpfile was not created")
	}
	name := s.tmpfile.Name()

	buf := make([]byte, 3)
	nn := 0
	nn, err = s.ReadAt(buf, 2)
	if err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if nn != 3 || string(buf) != "cde" {
		t.Fatalf("ReadAt() = (%d, %q), want (3, %q)", nn, string(buf), "cde")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temp file still exists or unexpected stat error: %v", err)
	}
	if got := s.Size(); got != 0 {
		t.Fatalf("Size() after Close = %d, want 0", got)
	}
}

func TestLimitedStorePrimaryOnly(t *testing.T) {
	s := NewLimitedStore(NewStoreMemory(), 8, nil)

	n, err := s.ReadFrom(bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if n != 5 {
		t.Fatalf("ReadFrom() n = %d, want 5", n)
	}

	buf := make([]byte, 5)
	nn, err := s.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if nn != 5 || string(buf) != "hello" {
		t.Fatalf("ReadAt() = (%d, %q), want (5, %q)", nn, string(buf), "hello")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestLimitedStoreLimitReachedWithoutSecondary(t *testing.T) {
	s := NewLimitedStore(NewStoreMemory(), 4, nil)

	n, err := s.ReadFrom(bytes.NewBufferString("hello"))
	if !errors.Is(err, ErrStoreLimit) {
		t.Fatalf("ReadFrom() error = %v, want %v", err, ErrStoreLimit)
	}
	if n != 4 {
		t.Fatalf("ReadFrom() n = %d, want 4", n)
	}
}

func TestLimitedStoreFallsBackToSecondary(t *testing.T) {
	s := NewLimitedStore(NewStoreMemory(), 4, NewStoreFile())

	n, err := s.ReadFrom(bytes.NewBufferString("hello world"))
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if n != 11 {
		t.Fatalf("ReadFrom() n = %d, want 11", n)
	}

	buf := make([]byte, 11)
	nn, err := s.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if nn != 11 || string(buf) != "hello world" {
		t.Fatalf("ReadAt() = (%d, %q), want (11, %q)", nn, string(buf), "hello world")
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestNewDefaultStore(t *testing.T) {
	s := NewDefaultStore()
	defer s.Close()

	n, err := s.ReadFrom(bytes.NewBufferString("abc"))
	if err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}
	if n != 3 {
		t.Fatalf("ReadFrom() n = %d, want 3", n)
	}

	buf := make([]byte, 3)
	nn, err := s.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt() error = %v", err)
	}
	if nn != 3 || string(buf) != "abc" {
		t.Fatalf("ReadAt() = (%d, %q), want (3, %q)", nn, string(buf), "abc")
	}
}
