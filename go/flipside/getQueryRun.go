package flipside

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"
)

// getQueryRunRequestParams are the parameters of the flipside request to get information about an existing query run.
type getQueryRunRequestParams struct {
	QueryRunID QueryRunID `json:"queryRunId"`
}

// getQueryRunResponse is the response payload of a getQueryRun request.
type getQueryRunResponse struct {
	QueryRun QueryRun `json:"queryRun"`
}

// GetQueryRun retrieves the details of a specific query run identified by QueryRunID.
// The QueryRunID is returned by CreateQueryRun.
func (cfg *Config) GetQueryRun(ctx context.Context, id QueryRunID) (*QueryRun, error) {
	resp, err := submitParamsAndParseResults[getQueryRunRequestParams, getQueryRunResponse](
		ctx, cfg, "getQueryRun", []getQueryRunRequestParams{
			{
				QueryRunID: id,
			},
		})
	if err != nil {
		return nil, err
	}
	return &resp.QueryRun, nil
}

// AwaitQueryRunExecution waits for a specific query run to complete, i.e. until its state changes.
// The state is checked with exponential backoff.
func (cfg *Config) AwaitQueryRunExecution(ctx context.Context, queryRunId QueryRunID, initialBackoff time.Duration, backoffFactor float64) (*QueryRun, error) {
	run, err := cfg.GetQueryRun(ctx, queryRunId)
	if err != nil {
		return nil, fmt.Errorf("getQueryRun(ctx, %q): %v", queryRunId, err)
	}

	glog.Infof("Query %s status: %v", queryRunId, run.State)

	// reverse engineered from: https://github.com/FlipsideCrypto/sdk/blob/main/js/src/types/query-status.type.ts
	switch run.State {
	case "QUERY_STATE_SUCCESS", "QUERY_STATE_FAILED", "QUERY_STATE_CANCELED":
		return run, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(initialBackoff):
		return cfg.AwaitQueryRunExecution(ctx, queryRunId, time.Duration(backoffFactor*float64(initialBackoff.Nanoseconds()))*time.Nanosecond, backoffFactor)
	}
}

// AwaitQueryRunSuccess waits for a specific query run to complete successfully.
// Unsuccessful queries will return an error.
func (cfg *Config) AwaitQueryRunSuccess(ctx context.Context, queryRunId QueryRunID, initialBackoff time.Duration, backoffFactor float64) error {
	run, err := cfg.AwaitQueryRunExecution(ctx, queryRunId, initialBackoff, backoffFactor)
	if err != nil {
		return err
	}

	if run.State != "QUERY_STATE_SUCCESS" {
		return fmt.Errorf("Query %s unsuccessful: %+v", queryRunId, run)
	}

	return nil
}
