package truenas

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeUnmarshalJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{
			name:  "two-digit day",
			input: `"Fri Mar 27 16:03:28 2026"`,
			want:  time.Date(2026, time.March, 27, 16, 3, 28, 0, time.UTC),
		},
		{
			name:  "single-digit day",
			input: `"Wed Jan  1 00:00:00 2025"`,
			want:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "invalid format",
			input:   `"2026-03-27T16:03:28Z"`,
			wantErr: true,
		},
		{
			name:    "not a string",
			input:   `12345`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got Time
			err := json.Unmarshal([]byte(tc.input), &got)
			if (err != nil) != tc.wantErr {
				t.Fatalf("UnmarshalJSON(%s) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			}
			if !tc.wantErr && !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got.Time, tc.want)
			}
		})
	}
}
