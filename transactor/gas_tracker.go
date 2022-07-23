package transactor

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

// GasTracker provides methods for gas tracking
type GasTracker interface {
	// GetSuggestedGasPriceFromGasTracker retrieve the network's suggested gas price
	GetSuggestedGasPriceFromGasTracker(ctx context.Context) (*GasTrackerResponse, error)
}

// GasTrackerResponse contains gas price values in GWei,
//'blockNumber' tells what was the latest block mined when recommendation was made
//'blockTime' in second, which gives average block time of network
type GasTrackerResponse struct {
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

func (r GasTrackerResponse) String() string {
	marshal, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(marshal)
}

type polygonGasTracker struct {
	gasTrackerURL string
}

func NewPolygonGasTracker(url string) GasTracker {
	return polygonGasTracker{gasTrackerURL: url}
}

func (o polygonGasTracker) GetSuggestedGasPriceFromGasTracker(ctx context.Context) (*GasTrackerResponse, error) {
	resp, err := http.Get(o.gasTrackerURL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrFailToGetResponseFromGasTracker, err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result GasTrackerResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	log.Ctx(ctx).Info().Str("response", result.String()).Msg("got from gas tracker")
	return &result, nil
}
