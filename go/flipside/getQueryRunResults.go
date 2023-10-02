package flipside

import (
	"context"
	"fmt"
	"time"
)

type requestPage struct {
	Number int `json:"number"`
	Size   int `json:"size"`
}

// getQueryRunResultsRequestParams are the parameters of the flipside request to fetch the results of a completed query run.
type getQueryRunResultsRequestParams struct {
	QueryRunID QueryRunID  `json:"queryRunId"`
	Format     string      `json:"format"`
	Page       requestPage `json:"page"`
}

// ResultsPage is part of QueryRunResults and holds pagination information.
type ResultsPage struct {
	CurrentPageNumber int `json:"currentPageNumber"`
	CurrentPageSize   int `json:"currentPageSize"`
	TotalRows         int `json:"totalRows"`
	TotalPages        int `json:"totalPages"`
}

// QueryRunResults is the response payload of a getQueryRunResults request and holds the execution results of a query.
type QueryRunResults[T any] struct {
	ColumnNames          []string    `json:"columnNames"`
	ColumnTypes          []string    `json:"columnTypes"`
	Rows                 []T         `json:"rows"`
	Page                 ResultsPage `json:"page"`
	SQL                  string      `json:"sql"`
	Format               string      `json:"format"`
	OriginalQueryRun     QueryRun    `json:"originalQueryRun"`
	RedirectedToQueryRun *QueryRun   `json:"redirectedToQueryRun"`
}

// GetQueryRunResults retrieves the results of a completed query run with pagination
func GetQueryRunResults[T any](ctx context.Context, cfg *Config, queryRunId QueryRunID, pageNumber int) (*QueryRunResults[T], error) {
	ret, err := submitParamsAndParseResults[getQueryRunResultsRequestParams, QueryRunResults[T]](
		ctx, cfg, "getQueryRunResults", []getQueryRunResultsRequestParams{
			{
				QueryRunID: queryRunId,
				Format:     "json", // TODO change this to json and unmarshal directly
				Page: requestPage{
					Number: pageNumber,
					Size:   100000,
				},
			},
		})
	if err != nil {
		return nil, fmt.Errorf("submitParamsAndDecode[GetQueryRunResultsRequestParams, GetQueryRunResultsResponse](ctx, cfg, \"createQueryRun\", []QueryRunResultsRequest{...}): %v", err)
	}

	return ret, nil
}

// FetchQueryResults is a convenience wrapper that waits until a given query succeeds and fetches all its results.
// The query queryRunId is returned by CreateQueryRun.
// Unfortunately golang does not allow generic types on methods, so we had to make this a free function instead.
func FetchQueryResults[T any](ctx context.Context, cfg *Config, queryRunId QueryRunID) ([]*QueryRunResults[T], error) {
	// TODO expose this to the user
	initialBackoff := 1 * time.Second
	backoffFactor := 1.2
	if err := cfg.AwaitQueryRunSuccess(ctx, queryRunId, initialBackoff, backoffFactor); err != nil {
		return nil, fmt.Errorf("cfg.awaitQueryRun(ctx, %q, ..): %v", queryRunId, err)
	}

	var results []*QueryRunResults[T]
	numPages := 1
	for i := 1; i <= numPages; i++ {
		res, err := GetQueryRunResults[T](ctx, cfg, queryRunId, i)
		if err != nil {
			return nil, fmt.Errorf("%T.GetQueryRunResults(ctx, %q, page=%d): %v", cfg, queryRunId, i, err)
		}
		results = append(results, res)
		numPages = res.Page.TotalPages
	}

	return results, nil
}
