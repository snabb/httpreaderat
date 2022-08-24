package contentrange_test

import (
	"log"
	"net/http"
	"testing"

	"github.com/snabb/httpreaderat/pkg/contentrange"
)

func TestParse(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name       string
		args       args
		wantFirst  int64
		wantLast   int64
		wantLength int64
		wantErr    bool
	}{
		{
			name: "full",
			args: args{
				str: "bytes 42-1233/1234",
			},
			wantFirst:  42,
			wantLast:   1233,
			wantLength: 1234,
			wantErr:    false,
		},
		{
			name: "size unknown",
			args: args{
				str: "bytes 42-1233/*",
			},
			wantFirst:  42,
			wantLast:   1233,
			wantLength: -1,
			wantErr:    false,
		},
		{
			name: "wildcard range",
			args: args{
				str: "bytes */1234",
			},
			wantFirst:  -1,
			wantLast:   -1,
			wantLength: 1234,
			wantErr:    false,
		},
		{
			name: "bad unit",
			args: args{
				str: "banana 200-1000/67589",
			},
			wantFirst:  -1,
			wantLast:   -1,
			wantLength: -1,
			wantErr:    true,
		},
		{
			name: "bad field",
			args: args{
				str: "bytes 0/67589",
			},
			wantFirst:  -1,
			wantLast:   -1,
			wantLength: -1,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFirst, gotLast, gotLength, err := contentrange.Parse(tt.args.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotFirst != tt.wantFirst {
				t.Errorf("Parse() gotFirst = %v, want %v", gotFirst, tt.wantFirst)
			}
			if gotLast != tt.wantLast {
				t.Errorf("Parse() gotLast = %v, want %v", gotLast, tt.wantLast)
			}
			if gotLength != tt.wantLength {
				t.Errorf("Parse() gotLength = %v, want %v", gotLength, tt.wantLength)
			}
		})
	}
}

func ExampleParse() {
	// fake http response
	res := http.Response{}
	res.Header.Add("Content-Range", "bytes 42-1233/1234")

	// get header and parse
	first, last, length, err := contentrange.Parse(res.Header.Get("Content-Range"))
	if err != nil {
		log.Panicf("can't parse content-range: %v", err)
	}

	log.Printf("%d, %d, %d", first, last, length)
}
