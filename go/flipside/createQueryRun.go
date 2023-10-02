package flipside

import (
	"context"
	"encoding/json"
	"time"
)

// Tags add metadata to a query run.
type Tags = map[string]string

// Hours is a duration that is serialized as float representing hours.
type Hours time.Duration

func (h Hours) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(h).Hours())
}

// Minutes is a duration that is serialized as float representing minutes.
type Minutes time.Duration

func (m Minutes) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(m).Minutes())
}

// createQueryRunRequestParams are the parameters of the flipside request to create a new query run.
type createQueryRunRequestParams struct {
	SQL          string  `json:"sql"`
	ResultsTTL   Hours   `json:"resultTTLHours"`
	MaxAge       Minutes `json:"maxAgeMinutes"`
	Tags         Tags    `json:"tags"`
	DataSource   string  `json:"dataSource"`
	DataProvider string  `json:"dataProvider"`
}

// QueryRequest is part of the response of a query run creation and identifies the request to create a new query.
type QueryRequest struct {
	ID                string     `json:"id"`
	SQLStatementID    string     `json:"sqlStatementId"`
	UserID            string     `json:"userId"`
	Tags              Tags       `json:"tags"`
	MaxAgeMinutes     Minutes    `json:"maxAgeMinutes"`
	ResultsTTLHours   Hours      `json:"resultTTLHours"`
	UserSkipCache     bool       `json:"userSkipCache"`
	TriggeredQueryRun bool       `json:"triggeredQueryRun"`
	QueryRunID        QueryRunID `json:"queryRunId"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// QueryRun is part of the response of a query run creation and identifies the new query that has been created.
type QueryRun struct {
	ID                    QueryRunID  `json:"id"`
	SQLStatementID        string      `json:"sqlStatementId"`
	State                 string      `json:"state"`
	Path                  string      `json:"path"`
	FileCount             int         `json:"fileCount"`
	LastFileNumber        *int        `json:"lastFileNumber"`
	FileNames             string      `json:"fileNames"`
	ErrorName             string      `json:"errorName"`
	ErrorMessage          interface{} `json:"errorMessage"`
	ErrorData             interface{} `json:"errorData"`
	DataSourceQueryID     string      `json:"dataSourceQueryId"`
	DataSourceSessionID   string      `json:"dataSourceSessionId"`
	StartedAt             time.Time   `json:"startedAt"`
	QueryRunningEndedAt   time.Time   `json:"queryRunningEndedAt"`
	QueryStreamingEndedAt time.Time   `json:"queryStreamingEndedAt"`
	EndedAt               time.Time   `json:"endedAt"`
	RowCount              int         `json:"rowCount"`
	TotalSize             string      `json:"totalSize"`
	Tags                  Tags        `json:"tags"`
	DataSourceID          string      `json:"dataSourceId"`
	UserID                string      `json:"userId"`
	CreatedAt             time.Time   `json:"createdAt"`
	UpdatedAt             time.Time   `json:"updatedAt"`
	ArchivedAt            time.Time   `json:"archivedAt"`
}

// SQLStatement is part of the response of a query run creation and identifies the SQL statement that will be executed as part of the run.
type SQLStatement struct {
	ID             string      `json:"id"`
	StatementHash  string      `json:"statementHash"`
	SQL            string      `json:"sql"`
	ColumnMetadata interface{} `json:"columnMetadata"`
	UserID         string      `json:"userId"`
	Tags           Tags        `json:"tags"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

type CreateQueryRunResponse struct {
	QueryRequest QueryRequest `json:"queryRequest"`
	QueryRun     QueryRun     `json:"queryRun"`
	SQLStatement SQLStatement `json:"sqlStatement"`
}

// CreateQueryRun submits an SQL query to flipside and creates a new query run.
// The query run is not executed immediately but will be queued for execution. Information about the query run can be retrieved with GetQueryRun.
// The query run will be created with the default data source, data provider, results TTL and max age.
func (cfg *Config) CreateQueryRun(ctx context.Context, sql string) (*CreateQueryRunResponse, error) {
	return submitParamsAndParseResults[createQueryRunRequestParams, CreateQueryRunResponse](
		ctx, cfg, "createQueryRun", []createQueryRunRequestParams{
			{
				SQL:        sql,
				ResultsTTL: Hours(10 * time.Hour),
				MaxAge:     Minutes(10 * time.Minute),
				Tags: map[string]string{
					"env": "proof",
				},
				DataSource:   "snowflake-default",
				DataProvider: "flipside",
			},
		})
}
