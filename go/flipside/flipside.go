// Package flipside provides convenience wrappers to work with flipside.xyz's REST API.
package flipside

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gocarina/gocsv"
	"github.com/golang/glog"
)

const apiURL = "https://node-api.flipsidecrypto.com/queries"

// Config configures the flipside wrapper.
type Config struct {
	APIKey        string
	BackoffFactor float64
}

type querySubmissionRequest struct {
	SQL string  `json:"sql"`
	TTL minutes `json:"ttlMinutes"`
}

type minutes time.Duration

func (m minutes) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(m).Minutes())
}

// QuerySubmissionResponse is returned by flipside after submitting a query and
// contains the query token that can be used to retrieve the results.
type QuerySubmissionResponse struct {
	Token string `json:"token"`
}

func (cfg *Config) addHeaders(r *http.Request) {
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("x-api-key", cfg.APIKey)
}

// SubmitQuery submits an SQL query to flipside.
func (cfg *Config) SubmitQuery(ctx context.Context, sql string) (*QuerySubmissionResponse, error) {
	query := querySubmissionRequest{
		SQL: sql,
		TTL: minutes(15 * time.Minute),
	}

	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(query); err != nil {
		return nil, fmt.Errorf("json.NewEncoder(w).Encode(%T): %v", query, err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		return nil, fmt.Errorf(`http.NewRequest("POST", [apiURL], [json]): %v`, err)
	}
	cfg.addHeaders(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do(%+v): %v", req, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("HTTP %d: io.ReadAll([resp.Body]): %v", resp.StatusCode, err)
		}

		return nil, fmt.Errorf("HTTP %d: %v", resp.StatusCode, string(body))
	}

	ret := new(QuerySubmissionResponse)
	if err := json.NewDecoder(resp.Body).Decode(ret); err != nil {
		return nil, fmt.Errorf("json.NewDecoder(resp.Body).Decode(%T): %v", ret, err)
	}

	return ret, nil
}

// QueryExecutionResponse encodes flipside's response for a query execution.
type QueryExecutionResponse struct {
	Results      [][]interface{} `json:"results"`
	ColumnLabels []string        `json:"columnLabels"`
	ColumnTypes  []string        `json:"columnTypes"`
	Status       string          `json:"status"`
	PageNumber   int             `json:"pageNumber"`
	PageSize     int             `json:"pageSize"`
	StartedAt    time.Time       `json:"startedAt"`
	EndedAt      time.Time       `json:"endedAt"`
}

// FetchQueryResults fetches the query results for a previously submitted query.
// The query token is returned by the SubmitQuery.
func (cfg *Config) FetchQueryResults(ctx context.Context, token string, pageNumber int) (*QueryExecutionResponse, error) {
	return cfg.fetchQueryResults(ctx, token, pageNumber, time.Second)
}

func (cfg *Config) fetchQueryResults(ctx context.Context, token string, pageNumber int, backoff time.Duration) (*QueryExecutionResponse, error) {
	url, err := url.JoinPath(apiURL, token)
	if err != nil {
		return nil, fmt.Errorf("url.JoinPath(%q, %q): %v", apiURL, token, err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf(`http.NewRequest("POST", [apiURL], nil): %v`, err)
	}
	cfg.addHeaders(req)

	q := req.URL.Query()
	q.Add("pageNumber", fmt.Sprintf("%d", pageNumber))
	q.Add("pageSize", "100000")
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do(%+v): %v", req, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("HTTP %d: io.ReadAll([resp.Body]): %v", resp.StatusCode, err)
		}

		return nil, fmt.Errorf("HTTP %d: %v", resp.StatusCode, string(body))
	}

	exResp := new(QueryExecutionResponse)
	if err := json.NewDecoder(resp.Body).Decode(&exResp); err != nil {
		return nil, fmt.Errorf("json.NewDecoder(resp.Body).Decode(%T): %v", exResp, err)
	}
	glog.Infof("Query %s status: %v", token, exResp.Status)

	if exResp.Status == "finished" {
		return exResp, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(backoff):
		return cfg.fetchQueryResults(ctx, token, pageNumber, time.Duration(cfg.BackoffFactor*float64(backoff.Nanoseconds()))*time.Nanosecond)
	}
}

// WriteCSV writes flipside query data as CSV.
// This uses `QueryExecutionResponse.ColumnLabels` as headers and `QueryExecutionResponse.Results` as rows.
func (r *QueryExecutionResponse) WriteCSV(w io.Writer) error {
	c := csv.NewWriter(w)
	if err := c.Write(r.ColumnLabels); err != nil {
		return fmt.Errorf("%T.Write([labels]): %v", c, err)
	}

	for _, row := range r.Results {
		var d []string
		for _, e := range row {
			d = append(d, fmt.Sprint(e))
		}
		if err := c.Write(d); err != nil {
			return fmt.Errorf("%T.Write([data]): %v", c, err)
		}
	}

	c.Flush()
	if err := c.Error(); err != nil {
		return fmt.Errorf("%T.Flush(): %v", c, err)
	}

	return nil
}

// Unmarshal parses the raw flipside query results and writes it into the given pointer.
// The data is parsed by creating an intermediate CSV representation (using `WriteCSV`) and unmarshalling this with `gocsv`.
// CSV struct tags are thus used to match columns as labelled in `QueryExecutionResponse.ColumnLabels`.
func (resp *QueryExecutionResponse) Unmarshal(v any) (retErr error) {
	cr, cw := io.Pipe()
	go func() {
		if err := resp.WriteCSV(cw); err != nil {
			retErr = fmt.Errorf("%w; %T.WriteCSV(%T): %v", retErr, resp, cw, err)
		}
		if err := cw.Close(); err != nil {
			retErr = fmt.Errorf("%w; %T.Close(): %v", retErr, cw, err)
		}
	}()

	if err := gocsv.Unmarshal(cr, v); err != nil {
		return fmt.Errorf("gocsv.Unmarshal(%T, %T): %v", cr, v, err)
	}
	return nil
}
