package tokens

import (
	"reflect"
	"testing"
)

func TestUnits(t *testing.T) {
	for _, test := range []struct {
		name   string
		tokens []string
		want   []Unit
	}{
		{
			name:   "subwords and punctuation",
			tokens: []string{" hello", "wor", "ld", ".", " next"},
			want:   []Unit{{Start: 0, End: 3}, {Start: 4, End: 4}},
		},
		{
			name:   "language without spaces",
			tokens: []string{"你", "好", "。", "吗"},
			want:   []Unit{{Start: 0, End: 0}, {Start: 1, End: 2}, {Start: 3, End: 3}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Units(test.tokens); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Units() = %+v, want %+v", got, test.want)
			}
		})
	}
}
