package loggers

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/DIMO-Network/edge-network/internal/queue"
	"github.com/rs/zerolog"

	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestExecutePID(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()
	// mock http
	v := `|-\n7e8101b62f190314654\n7e8214557314350334e\n7e8224b453638353933\n7e82300000000000000`
	respJSON := fmt.Sprintf(`{"value": "%s"}`, v)
	url := fmt.Sprintf("%s/dongle/%s/execute_raw", "http://192.168.4.1:9000", unitID.String())
	httpmock.RegisterResponder(http.MethodPost, url, httpmock.NewStringResponder(200, respJSON))

	qs := queue.NewDiskStorageQueue(unitID)
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()
	vl := NewPIDLogger(unitID, qs, logger)

	err := vl.ExecutePID(uint32(2015), uint32(1), uint32(47), "", "", "vin")
	require.NoError(t, err)
}

func TestExecutePIDWithError(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	unitID := uuid.New()
	// mock http
	v := ``
	respJSON := fmt.Sprintf(`{"value": "%s"}`, v)
	url := fmt.Sprintf("%s/dongle/%s/execute_raw", "http://192.168.4.1:9000", unitID.String())
	httpmock.RegisterResponder(http.MethodPost, url, httpmock.NewStringResponder(500, respJSON))
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("app", "edge-network").
		Logger()
	qs := queue.NewDiskStorageQueue(unitID)
	vl := NewPIDLogger(unitID, qs, logger)

	err := vl.ExecutePID(uint32(2015), uint32(1), uint32(47), "", "", "vin")
	require.Error(t, err)
}