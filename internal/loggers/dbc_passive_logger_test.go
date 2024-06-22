package loggers

import (
	_ "embed"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

//go:embed test_gm120.dbc
var testgm120dbc string

func Test_dbcPassiveLogger_parseDBCHeaders(t *testing.T) {
	testLogger := zerolog.New(os.Stdout).Output(zerolog.ConsoleWriter{Out: os.Stdout})

	tests := []struct {
		name    string
		dbcFile string
		want    []dbcFilter
	}{
		{
			name:    "gm odometer",
			dbcFile: testgm120dbc,
			want: []dbcFilter{
				{
					header:  288,
					formula: `7|32@0+ (0.015625,0) [0|67108863.984375] "km"  Vector_XXX`,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dpl := &dbcPassiveLogger{
				logger: testLogger,
			}
			parsed, err := dpl.parseDBCHeaders(tt.dbcFile)
			assert.NoError(t, err)
			assert.Equal(t, tt.want[0], parsed[0])
		})
	}
}
