package reader

import (
	"reflect"
	"testing"
)

func TestLogfmt(t *testing.T) {
	nf := readOne(t, "data.logfmt", noInferOpts(FormatLogfmt))
	checkGrid(t, nf.Frame,
		[]string{"level", "msg", "count", "extra", "debug"},
		[][]any{
			{"info", "hello world", "2", nil, nil},
			{"warn", `a "quoted" word`, nil, "x", "true"},
			{"error", nil, "7", nil, nil},
		})
}

func TestParseLogfmtLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantKeys []string
		wantVals map[string]any
		wantErr  bool
	}{
		{
			name:     "bare pairs",
			line:     "a=1 b=two",
			wantKeys: []string{"a", "b"},
			wantVals: map[string]any{"a": "1", "b": "two"},
		},
		{
			name:     "quoted value with spaces and escapes",
			line:     `msg="hello \"world\"" x=1`,
			wantKeys: []string{"msg", "x"},
			wantVals: map[string]any{"msg": `hello "world"`, "x": "1"},
		},
		{
			name:     "bare key is true",
			line:     "verbose a=1",
			wantKeys: []string{"verbose", "a"},
			wantVals: map[string]any{"verbose": "true", "a": "1"},
		},
		{
			name:     "empty value",
			line:     "a= b=2",
			wantKeys: []string{"a", "b"},
			wantVals: map[string]any{"a": "", "b": "2"},
		},
		{
			name:    "unterminated quote",
			line:    `a="oops`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, vals, err := parseLogfmtLine(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(keys, tt.wantKeys) {
				t.Errorf("keys = %v, want %v", keys, tt.wantKeys)
			}
			if !reflect.DeepEqual(vals, tt.wantVals) {
				t.Errorf("vals = %v, want %v", vals, tt.wantVals)
			}
		})
	}
}

func TestLogfmtIgnoreErrors(t *testing.T) {
	src := writeTemp(t, "bad.logfmt", "a=1\nb=\"broken\na=2\n")
	r, _ := For(FormatLogfmt)

	opt := noInferOpts(FormatLogfmt)
	if _, err := r.Read(src, opt); err == nil {
		t.Fatal("want error without IgnoreErrors")
	}

	opt.IgnoreErrors = true
	frames, err := r.Read(src, opt)
	if err != nil {
		t.Fatal(err)
	}
	checkGrid(t, frames[0].Frame,
		[]string{"a"},
		[][]any{{"1"}, {"2"}})
}
