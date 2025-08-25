package loggers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			name: "simulator",
			hexValue: `|-
  7E8101462F190574155
  7E8215247423852324C
  7E8224E303036323232`,
			wantVin:         "WAURGB8R2LN006222",
			wantVinStartPos: 3,
			wantErr:         false,
		},
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
		{
			name: "2008_Nissan_Altima_7DF_09_02_P6 stdPID",
			hexValue: `|-
  7e81014490201314e34
  7e821414c3231453738
  7e8224e353139313437`,
			wantVin:         "1N4AL21E78N519147",
			wantVinStartPos: 1,
			wantErr:         false,
		},
		{
			name: "2020_Chevrolet_Silverado_1500_7e0_09_02_P6 stdPID",
			hexValue: `|-
  7e81014490201314743
  7e821525942454b324c
  7e8225a333239323338`,
			wantVin:         "1GCRYBEK2LZ329238",
			wantVinStartPos: 1,
			wantErr:         false,
		},
		{
			name: "2019_Volvo_XC60_7df_09_02_P6 stdPID",
			hexValue: `|-
  7e81014490201595634
  7e821425230444d304a
  7e82231303232353139`,
			wantVin:         "YV4BR0DM0J1022519",
			wantVinStartPos: 1,
			wantErr:         false,
		},
		{
			name: "2019_Honda_CR-V_18DB33F1_09_02_P7 stdPID",
			hexValue: `|-
  18daf1101014490201354a36
  18daf110215257324838394b
  18daf110224c303132333032`,
			wantVin:         "5J6RW2H89KL012302",
			wantVinStartPos: 1,
			wantErr:         false,
		},
		{
			name: "2018_Ford_F-150_7DF_09_02 stdPID",
			hexValue: `|-
  7e81014490201314654
  7e8214557314342324a
  7e8224b443338353331`,
			wantVin:         "1FTEW1CB2JKD38531",
			wantVinStartPos: 1,
			wantErr:         false,
		},
		{
			name: "2017_Hyundai_Tucson_7DF_09_02 stdPID",
			hexValue: `|-
  7e810144902014b4d38
  7e8214a334341323948
  7e82255323938343437`,
			wantVin:         "KM8J3CA29HU298447",
			wantVinStartPos: 1,
			wantErr:         false,
		},
		{
			name: " 2015_Dacia_Sandero_7DF_22_F190_P6 UDS",
			hexValue: `|-
  7e8101462f190555531
  7e82135534447473535
  7e82230343237313333`,
			wantVin:         "UU15SDGG550427133",
			wantVinStartPos: 3,
			wantErr:         false,
		},
		{
			name: "Dacia error",
			hexValue: `|-
7e81014490201202020
7e82120202020202020
7e82220202020202020`,
			wantErr: true,
		},
		{
			name: "Mateusz and Steve example VIN UDS, vin_18DB33F1_UDS",
			hexValue: `|-
18DAF111101862F19011354A
18DAF1112136524534483734
18DAF11122424C3035363039
18DAF1112337000000555555
  `,
			wantVin: "5J6RE4H74BL056097",
			// need to use the request pid to help decode to find start position, 354A is start in frame 1, 37 is last byte we want in frame 4, the rest is padding.
			wantVinStartPos: 10,
			wantErr:         false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frames := strings.Split(tt.hexValue, "\n")
			gotVin, gotStartPos, err := extractVIN(frames)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractVIN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantVin, gotVin)
			assert.Equal(t, tt.wantVinStartPos, gotStartPos)
		})
	}
}

func TestGetVIN(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()
	const testVIN = "1FTEW1CP3NKE68593"
	// mock http
	v := `|-\n7e8101b62f190314654\n7e8214557314350334e\n7e8224b453638353933\n7e82300000000000000`
	respJSON := fmt.Sprintf(`{"value": "%s", "_stamp": "2024-02-29T17:17:30.534861"}`, v)
	url := fmt.Sprintf("%s/dongle/%s/execute_raw", "http://192.168.4.1:9000", unitID.String())
	httpmock.RegisterResponder(http.MethodPost, url, httpmock.NewStringResponder(200, respJSON))

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	vl := NewVINLogger(logger)

	vinResp, err := vl.GetVIN(unitID, nil)
	require.NoError(t, err)
	assert.Equal(t, "6", vinResp.Protocol)
	assert.Equal(t, testVIN, vinResp.VIN)
}

func TestGetVIN_withQueryName(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()
	const testVIN = "1FTEW1CP3NKE68593"
	// mock http
	v := `|-\n7e8101b62f190314654\n7e8214557314350334e\n7e8224b453638353933\n7e82300000000000000`
	respJSON := fmt.Sprintf(`{"value": "%s", "_stamp": "2024-02-29T17:17:30.534861"}`, v)
	url := fmt.Sprintf("%s/dongle/%s/execute_raw", "http://192.168.4.1:9000", unitID.String())
	httpmock.RegisterResponder(http.MethodPost, url, httpmock.NewStringResponder(200, respJSON))

	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()

	vl := NewVINLogger(logger)
	qn := "vin_18DB33F1_09_02"
	vinResp, err := vl.GetVIN(unitID, &qn)
	require.NoError(t, err)
	assert.Equal(t, "7", vinResp.Protocol)
	assert.Equal(t, testVIN, vinResp.VIN)
}
