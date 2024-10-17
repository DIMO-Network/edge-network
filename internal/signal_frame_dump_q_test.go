package internal

import (
	"testing"
	"time"

	"github.com/DIMO-Network/edge-network/internal/models"
)

func TestDetermineJobDone(t *testing.T) {
	now := time.Now()

	testCases := []struct {
		desc  string
		input *models.CANDumpInfo
		want  bool
	}{
		{
			desc:  "nil CANDumpInfo",
			input: nil,
			want:  false,
		},
		{
			desc:  "DateExecuted is older than 30 days",
			input: &models.CANDumpInfo{DateExecuted: now.Add(-31 * 24 * time.Hour)},
			want:  false,
		},
		{
			desc:  "DateExecuted is equal to 30 days",
			input: &models.CANDumpInfo{DateExecuted: now.Add(-30 * 24 * time.Hour)},
			want:  true,
		},
		{
			desc:  "DateExecuted is less than 30 days",
			input: &models.CANDumpInfo{DateExecuted: now.Add(-29 * 24 * time.Hour)},
			want:  true,
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			if got := determineJobDone(tC.input); got != tC.want {
				t.Errorf("determineJobDone() = %v, want %v for input %v", got, tC.want, tC.input)
			}
		})
	}
}
