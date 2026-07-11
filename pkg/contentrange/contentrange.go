package contentrange

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrParse            = errors.New("content-range parse error")
	ErrUnsupportedUnit  = errors.New("unsupported unit")
	ErrUnsupportedField = errors.New("unsupported field")
)

// Parse parses the contents of a Content-Range header.
func Parse(str string) (first, last, length int64, err error) {
	fields := strings.Fields(str)
	if len(fields) == 0 {
		return -1, -1, -1, ErrParse
	}
	if fields[0] != "bytes" {
		return -1, -1, -1, ErrUnsupportedUnit
	}
	if len(fields) != 2 {
		return -1, -1, -1, ErrParse
	}

	rangeField, lengthField, ok := strings.Cut(fields[1], "/")
	if !ok || strings.Contains(lengthField, "/") {
		return -1, -1, -1, ErrParse
	}

	if rangeField == "*" {
		length, err := parseNonNegative(lengthField, "length")
		if err != nil {
			return -1, -1, -1, err
		}
		return -1, -1, length, nil
	}

	firstField, lastField, ok := strings.Cut(rangeField, "-")
	if !ok || strings.Contains(lastField, "-") {
		return -1, -1, -1, ErrUnsupportedField
	}
	first, err = parseNonNegative(firstField, "first")
	if err != nil {
		return -1, -1, -1, err
	}
	last, err = parseNonNegative(lastField, "last")
	if err != nil {
		return -1, -1, -1, err
	}
	if first > last {
		return -1, -1, -1, ErrUnsupportedField
	}

	length = -1
	if lengthField != "*" {
		length, err = parseNonNegative(lengthField, "length")
		if err != nil {
			return -1, -1, -1, err
		}
		if last >= length {
			return -1, -1, -1, ErrUnsupportedField
		}
	}
	return first, last, length, nil
}

func parseNonNegative(field, name string) (int64, error) {
	value, err := strconv.ParseInt(field, 10, 64)
	if err != nil || value < 0 {
		if err == nil {
			err = ErrUnsupportedField
		}
		return -1, fmt.Errorf("can't parse %s: %w", name, err)
	}
	return value, nil
}
