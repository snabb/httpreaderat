package contentrange

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

var (
	ErrParse            = errors.New("content-range parse error")
	ErrUnsupportedUnit  = errors.New("unsupported unit")
	ErrUnsupportedField = errors.New("unsupported field")
)

// Parse parse content of a Content-Range header.
func Parse(str string) (first, last, length int64, err error) {
	split := func(r rune) bool {
		if unicode.IsSpace(r) {
			return true
		}

		switch r {
		case '-', '/':
			return true
		}
		return false
	}

	fields := strings.FieldsFunc(str, split)

	if fields[0] != "bytes" {
		return -1, -1, -1, ErrUnsupportedUnit
	}

	if len(fields) == 4 {
		first, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("can't parse first: %w", err)
		}
		last, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("can't parse first: %w", err)
		}
		length := int64(-1)
		if fields[3] != "*" {
			length, err = strconv.ParseInt(fields[3], 10, 64)
			if err != nil {
				return -1, -1, -1, fmt.Errorf("can't parse length: %w", err)
			}
		}

		return first, last, length, nil
	}

	if len(fields) == 3 {
		if fields[1] != "*" {
			return -1, -1, -1, ErrUnsupportedField
		}

		length, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("can't parse length: %w", err)
		}

		return -1, -1, length, nil
	}

	return -1, -1, -1, ErrParse
}
