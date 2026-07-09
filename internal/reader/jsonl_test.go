package reader

import "testing"

func TestJSONLSkipsBlankLines(t *testing.T) {
	nf := readOne(t, "data.jsonl", noInferOpts(FormatJSONL))
	checkGrid(t, nf.Frame,
		[]string{"a", "b", "c"},
		[][]any{
			{int64(1), "x", nil},
			{int64(2), nil, true},
		})
}

func TestJSONLBadLine(t *testing.T) {
	r, _ := For(FormatJSONL)

	opt := noInferOpts(FormatJSONL)
	if _, err := r.Read(fixture(t, "bad.jsonl"), opt); err == nil {
		t.Fatal("want parse error without IgnoreErrors")
	}

	opt.IgnoreErrors = true
	frames, err := r.Read(fixture(t, "bad.jsonl"), opt)
	if err != nil {
		t.Fatal(err)
	}
	checkGrid(t, frames[0].Frame,
		[]string{"a"},
		[][]any{{int64(1)}, {int64(2)}})
}
