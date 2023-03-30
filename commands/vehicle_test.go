package commands

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_extractVIN(t *testing.T) {
	tests := []struct {
		name            string
		hexValue        string
		wantVin         string
		wantVinStartPos int
		wantErr         bool
	}{
		{
			name: "2022_Ford_F150_7DF_22_F190_P6 UDS",
			hexValue: `|-
  7e8101b62f190314654
  7e8214557314350334e
  7e8224b453638353933
  7e82300000000000000`,
			wantVin:         "1FTEW1CP3NKE68593",
			wantVinStartPos: 3,
			wantErr:         false,
		},
		{
			name: "2021_Subaru_Ascent_7DF_09_02_P6 stdPID",
			hexValue: `|-
  7e81014490201345334
  7e821574d414644344d
  7e82233343236333533`,
			wantVin:         "4S4WMAFD4M3426353",
			wantVinStartPos: 1, // this doesn't make sense, should be higher like 2 or 3
			wantErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVin, gotStartPos, err := extractVIN(tt.hexValue)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVIN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantVin, gotVin)
			assert.Equal(t, tt.wantVinStartPos, gotStartPos)
		})
	}
}
