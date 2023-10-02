package flipside

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWriteCSV(t *testing.T) {
	tests := []struct {
		name     string
		response QueryExecutionResponse
		want     string
	}{
		{
			name: "two columns",
			response: QueryExecutionResponse{
				ColumnLabels: []string{"A", "B"},
				Results:      [][]any{{0.1337, "foo"}, {42.1, "bar"}},
			},
			want: "A,B\n0.1337,foo\n42.1,bar\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := new(bytes.Buffer)
			if err := tt.response.WriteCSV(got); err != nil {
				t.Fatalf("%T.WriteCSV(%T): %v", tt.response, got, err)
			}

			if diff := cmp.Diff(tt.want, got.String()); diff != "" {
				t.Errorf("%T.WriteCSV(%T) diff (+got -want): %s", tt.response, got, diff)
			}
		})
	}
}

func TestUnmarshal(t *testing.T) {
	type MyType struct {
		A float64 `csv:"A"`
		B string  `csv:"B"`
	}

	tests := []struct {
		name     string
		response QueryExecutionResponse
		want     []MyType
	}{
		{
			name: "two columns",
			response: QueryExecutionResponse{
				ColumnLabels: []string{"A", "B"},
				Results:      [][]any{{0.1337, "foo"}, {42.1, "bar"}},
			},
			want: []MyType{
				{A: 0.1337, B: "foo"},
				{A: 42.1, B: "bar"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b []MyType
			if err := tt.response.Unmarshal(&b); err != nil {
				t.Fatalf("%T.Unmarshal(%T): %v", tt.response, &b, err)
			}

			if diff := cmp.Diff(tt.want, b); diff != "" {
				t.Errorf("%T.Unmarshal(%T) diff (+got -want): %s", tt.response, &b, diff)
			}
		})
	}
}
