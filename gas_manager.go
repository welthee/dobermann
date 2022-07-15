package dobermann

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"net/http"
)

var ErrFailToGetResponseFromGasTracker = errors.New("failed to get a response from the gas tracker")

type PolygonGasStationResponse struct {
	SafeLow struct {
		MaxPriorityFee float64 `json:"maxPriorityFee"`
		MaxFee         float64 `json:"maxFee"`
	} `json:"safeLow"`
	Standard struct {
		MaxPriorityFee float64 `json:"maxPriorityFee"`
		MaxFee         float64 `json:"maxFee"`
	} `json:"standard"`
	Fast struct {
		MaxPriorityFee float64 `json:"maxPriorityFee"`
		MaxFee         float64 `json:"maxFee"`
	} `json:"fast"`
	EstimatedBaseFee float64 `json:"estimatedBaseFee"`
	BlockTime        int     `json:"blockTime"`
	BlockNumber      int     `json:"blockNumber"`
}

func (r PolygonGasStationResponse) String() string {
	marshal, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(marshal)
}

type PolygonGasTracker struct {
	gasTrackerURL string
}

func NewPolygonGasTracker(url string) PolygonGasTracker {
	return PolygonGasTracker{gasTrackerURL: url}
}

func (o PolygonGasTracker) getSuggestedGasPriceFromGasTracker(ctx context.Context) (*PolygonGasStationResponse, error) {
	resp, err := http.Get(o.gasTrackerURL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrFailToGetResponseFromGasTracker, err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result PolygonGasStationResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	log.Info().Str("response", result.String()).Msg("got from gas tracker")
	return &result, nil
}
