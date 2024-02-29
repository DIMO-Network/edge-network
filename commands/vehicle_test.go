package commands

import "testing"

func Test_isValidHex(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want bool
	}{
		{
			hex:  "0x1A3F",
			want: true,
		},
		{
			hex:  "0X4D52",
			want: true,
		},
		{
			hex:  "7DF",
			want: true,
		},
		{
			hex:  "88Z1",
			want: false,
		},
		{
			hex:  "0x",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			if got := isValidHex(tt.hex); got != tt.want {
				t.Errorf("isValidHex() = %v, want %v", got, tt.want)
			}
		})
	}
}
