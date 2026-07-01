package output

import "testing"

func Test_DurFmt(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"zero is blank", 0, ""},
		{"negative is blank", -5, ""},
		{"sub-decisecond rounds to 0.0s", 30, "0.0s"},
		{"tenths round down", 440, "0.4s"},
		{"tenths round up", 460, "0.5s"},
		{"just under the second boundary", 949, "0.9s"},
		{"rounds up into whole seconds", 950, "1s"},
		{"one second", 1000, "1s"},
		{"whole seconds", 12000, "12s"},
		{"rounds to nearest second", 12499, "12s"},
		{"just under a minute", 59000, "59s"},
		{"minutes and seconds", 65000, "1m5s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := durFmt(tt.ms); got != tt.want {
				t.Errorf("durFmt(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}
