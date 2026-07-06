package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"train-status-app/backend/internal/model"
)

var ErrExternalAPI = errors.New("external api error")

const (
	trainStatusURL   = "https://api-public.odpt.org/api/v4/odpt:TrainInformation?odpt:operator=odpt.Operator:Toei"
	trainLocationURL = "https://api-public.odpt.org/api/v4/odpt:Train?odpt:operator=odpt.Operator:Toei"
)

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func fetch[T any](
	ctx context.Context,
	httpClient *http.Client,
	url string,
) ([]T, error) {

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		url,
		nil,
	)
	if err != nil {
		return nil, err
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, ErrExternalAPI
	}

	var data []T

	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) GetTrainStatus(
	ctx context.Context,
) ([]model.TrainStatus, error) {

	return fetch[model.TrainStatus](
		ctx,
		c.http,
		trainStatusURL,
	)
}

func (c *Client) GetTrainLocations(
	ctx context.Context,
) ([]model.TrainLocation, error) {

	return fetch[model.TrainLocation](
		ctx,
		c.http,
		trainLocationURL,
	)
}
