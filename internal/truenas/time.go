package truenas

import (
	"encoding/json"
	"fmt"
	"time"
)

// timeLayout is the time format used by TrueNAS,
// e.g. "Fri Mar 27 16:03:28 2026".
const timeLayout = "Mon Jan _2 15:04:05 2006"

// Time is a [time.Time] that can unmarshal TrueNAS date strings.
type Time struct{ time.Time }

func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(timeLayout, s)
	if err != nil {
		return fmt.Errorf("cannot parse %q as a TrueNAS time: %w", s, err)
	}
	t.Time = parsed
	return nil
}
