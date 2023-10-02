package flipside

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func newFlipsideMockServer(t *testing.T, nextResponse func() string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(nextResponse()))
	}))
	t.Cleanup(server.Close)

	return server
}

func timeFrom(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCreateQueryRun(t *testing.T) {
	tests := []struct {
		name        string
		apiResponse string
		want        *CreateQueryRunResponse
		wantErr     error
	}{
		{
			name: "ok",
			// taken from https://api-docs.flipsidecrypto.xyz/#de93c35e-49ad-48a5-8a21-a0b3fc9bec2a
			apiResponse: `{"jsonrpc":"2.0","id":1,"result":{"queryRequest":{"id":"clgnvu28c004zom0tauau46zc","sqlStatementId":"clgnvu24w004xom0tzu5r7g3v","userId":"clf8qd1eb0000jv08kbuw0dy4","tags":{"env":"test","source":"postman-demo"},"maxAgeMinutes":0,"resultTTLHours":1,"userSkipCache":true,"triggeredQueryRun":true,"queryRunId":"clgnvu25a004yom0tuaakp58q","createdAt":"2023-04-19T16:02:36.000Z","updatedAt":"2023-04-19T16:02:36.000Z"},"queryRun":{"id":"clgnvu25a004yom0tuaakp58q","sqlStatementId":"clgnvu24w004xom0tzu5r7g3v","state":"QUERY_STATE_READY","path":"2023/04/19/16/clgnvu25a004yom0tuaakp58q","fileCount":null,"lastFileNumber":null,"fileNames":null,"errorName":null,"errorMessage":null,"errorData":null,"dataSourceQueryId":null,"dataSourceSessionId":null,"startedAt":null,"queryRunningEndedAt":null,"queryStreamingEndedAt":null,"endedAt":null,"rowCount":null,"totalSize":null,"tags":{"env":"test","source":"postman-demo"},"dataSourceId":"clf90gwee0002jvbu63diaa8u","userId":"clf8qd1eb0000jv08kbuw0dy4","createdAt":"2023-04-19T16:02:36.000Z","updatedAt":"2023-04-19T16:02:36.000Z","archivedAt":null},"sqlStatement":{"id":"clgnvu24w004xom0tzu5r7g3v","statementHash":"3d41b46907d05dd97849e9f02e8aa9115f812763f080920c2785348ae593b0bb","sql":"SELECT date_trunc('hour', block_timestamp) as hourly_datetime, count(distinct tx_hash) as tx_count from ethereum.core.fact_transactions where block_timestamp >= getdate() - interval'1 month' group by 1 order by 1 desc","columnMetadata":null,"userId":"clf8qd1eb0000jv08kbuw0dy4","tags":{"env":"test","source":"postman-demo"},"createdAt":"2023-04-19T16:02:36.000Z","updatedAt":"2023-04-19T16:02:36.000Z"}}}`,
			want: &CreateQueryRunResponse{
				QueryRequest: QueryRequest{
					ID:                "clgnvu28c004zom0tauau46zc",
					SQLStatementID:    "clgnvu24w004xom0tzu5r7g3v",
					UserID:            "clf8qd1eb0000jv08kbuw0dy4",
					Tags:              Tags{"env": "test", "source": "postman-demo"},
					MaxAgeMinutes:     0,
					ResultsTTLHours:   1,
					UserSkipCache:     true,
					TriggeredQueryRun: true,
					QueryRunID:        "clgnvu25a004yom0tuaakp58q",
					CreatedAt:         timeFrom("2023-04-19T16:02:36.000Z"),
					UpdatedAt:         timeFrom("2023-04-19T16:02:36.000Z"),
				},
				QueryRun: QueryRun{
					ID:                    "clgnvu25a004yom0tuaakp58q",
					SQLStatementID:        "clgnvu24w004xom0tzu5r7g3v",
					State:                 "QUERY_STATE_READY",
					Path:                  "2023/04/19/16/clgnvu25a004yom0tuaakp58q",
					FileCount:             0,
					LastFileNumber:        nil,
					FileNames:             "",
					ErrorName:             "",
					ErrorMessage:          nil,
					ErrorData:             nil,
					DataSourceQueryID:     "",
					DataSourceSessionID:   "",
					StartedAt:             time.Time{},
					QueryRunningEndedAt:   time.Time{},
					QueryStreamingEndedAt: time.Time{},
					EndedAt:               time.Time{},
					RowCount:              0,
					TotalSize:             "",
					Tags:                  Tags{"env": "test", "source": "postman-demo"},
					DataSourceID:          "clf90gwee0002jvbu63diaa8u",
					UserID:                "clf8qd1eb0000jv08kbuw0dy4",
					CreatedAt:             timeFrom("2023-04-19T16:02:36.000Z"),
					UpdatedAt:             timeFrom("2023-04-19T16:02:36.000Z"),
					ArchivedAt:            time.Time{},
				},
				SQLStatement: SQLStatement{
					ID:             "clgnvu24w004xom0tzu5r7g3v",
					StatementHash:  "3d41b46907d05dd97849e9f02e8aa9115f812763f080920c2785348ae593b0bb",
					SQL:            "SELECT date_trunc('hour', block_timestamp) as hourly_datetime, count(distinct tx_hash) as tx_count from ethereum.core.fact_transactions where block_timestamp >= getdate() - interval'1 month' group by 1 order by 1 desc",
					ColumnMetadata: nil,
					UserID:         "clf8qd1eb0000jv08kbuw0dy4",
					Tags:           Tags{"env": "test", "source": "postman-demo"},
					CreatedAt:      timeFrom("2023-04-19T16:02:36.000Z"),
					UpdatedAt:      timeFrom("2023-04-19T16:02:36.000Z"),
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			server := newFlipsideMockServer(t, func() string { return tt.apiResponse })

			fs := Config{APIKey: "test", APIURL: server.URL}

			got, err := fs.CreateQueryRun(ctx, "")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateQueryRun() err: %v, wantErr: %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); tt.want != nil && diff != "" {
				t.Errorf("CreateQueryRun() diff (+got -want): %s", diff)
			}
		})
	}
}

func intPtr(x int) *int {
	return &x
}

func TestGetQueryRun(t *testing.T) {
	tests := []struct {
		name        string
		apiResponse string
		want        *QueryRun
		wantErr     error
	}{
		{
			name: "ok",
			// taken from https://api-docs.flipsidecrypto.xyz/#fddab00f-4420-44b1-9e95-ef81e6350847
			apiResponse: `{"jsonrpc":"2.0","id":1,"result":{"queryRun":{"id":"clg44olzq00cbn60tasvob5l2","sqlStatementId":"clg44oly200c9n60tviq17sng","state":"QUERY_STATE_SUCCESS","path":"2023/04/05/20/clg44olzq00cbn60tasvob5l2","fileCount":1,"lastFileNumber":1,"fileNames":"clg44olzq00cbn60tasvob5l2-consolidated-results.parquet","errorName":null,"errorMessage":null,"errorData":null,"dataSourceQueryId":null,"dataSourceSessionId":"17257398387030526","startedAt":"2023-04-05T20:14:55.000Z","queryRunningEndedAt":"2023-04-05T20:15:16.000Z","queryStreamingEndedAt":"2023-04-05T20:17:18.000Z","endedAt":"2023-04-05T20:17:18.000Z","rowCount":17000,"totalSize":"24904891","tags":{"sdk_package":"python","sdk_version":"1.0.2","sdk_language":"python"},"dataSourceId":"clf90gwee0002jvbu63diaa8u","userId":"clf8qd1eb0000jv08kbuw0dy4","createdAt":"2023-04-05T20:14:55.000Z","updatedAt":"2023-04-05T20:14:55.000Z","archivedAt":null},"redirectedToQueryRun":null}}`,
			want: &QueryRun{
				ID:                    "clg44olzq00cbn60tasvob5l2",
				SQLStatementID:        "clg44oly200c9n60tviq17sng",
				State:                 "QUERY_STATE_SUCCESS",
				Path:                  "2023/04/05/20/clg44olzq00cbn60tasvob5l2",
				FileCount:             1,
				LastFileNumber:        intPtr(1),
				FileNames:             "clg44olzq00cbn60tasvob5l2-consolidated-results.parquet",
				ErrorName:             "",
				ErrorMessage:          nil,
				ErrorData:             nil,
				DataSourceQueryID:     "",
				DataSourceSessionID:   "17257398387030526",
				StartedAt:             timeFrom("2023-04-05T20:14:55.000Z"),
				QueryRunningEndedAt:   timeFrom("2023-04-05T20:15:16.000Z"),
				QueryStreamingEndedAt: timeFrom("2023-04-05T20:17:18.000Z"),
				EndedAt:               timeFrom("2023-04-05T20:17:18.000Z"),
				RowCount:              17000,
				TotalSize:             "24904891",
				Tags:                  Tags{"sdk_package": "python", "sdk_version": "1.0.2", "sdk_language": "python"},
				DataSourceID:          "clf90gwee0002jvbu63diaa8u",
				UserID:                "clf8qd1eb0000jv08kbuw0dy4",
				CreatedAt:             timeFrom("2023-04-05T20:14:55.000Z"),
				UpdatedAt:             timeFrom("2023-04-05T20:14:55.000Z"),
				ArchivedAt:            time.Time{},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			server := newFlipsideMockServer(t, func() string { return tt.apiResponse })

			fs := Config{APIKey: "test", APIURL: server.URL}

			got, err := fs.GetQueryRun(ctx, "")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetQueryRun() err: %v, wantErr: %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); tt.want != nil && diff != "" {
				t.Errorf("GetQueryRun() diff (+got -want): %s", diff)
			}
		})
	}
}

type QueryRunResponses struct {
	numCalls  int
	responses []*QueryRun
}

func (r *QueryRunResponses) next() string {
	r.numCalls++

	if len(r.responses) == 0 {
		return ""
	}

	resp := r.responses[0]
	r.responses = r.responses[1:]

	b, err := json.Marshal(response[getQueryRunResponse]{
		Result: getQueryRunResponse{*resp},
	})
	if err != nil {
		panic(err)
	}

	return string(b)
}

func TestAwaitQueryRunExecution(t *testing.T) {
	ready := &QueryRun{
		ID:    "ready",
		State: "QUERY_STATE_READY",
	}
	running := &QueryRun{
		ID:    "running",
		State: "QUERY_STATE_RUNNING",
	}
	success := &QueryRun{
		ID:    "success",
		State: "QUERY_STATE_SUCCESS",
	}
	failed := &QueryRun{
		ID:    "failed",
		State: "QUERY_STATE_FAILED",
	}
	cancelled := &QueryRun{
		ID:    "cancelled",
		State: "QUERY_STATE_CANCELED",
	}

	tests := []struct {
		name         string
		apiResponses []*QueryRun
		want         *QueryRun
		wantTrials   int
		wantErr      error
	}{
		{
			name: "success",
			apiResponses: []*QueryRun{
				ready,
				running,
				success,
			},
			want:       success,
			wantTrials: 3,
		},
		{
			name: "failed",
			apiResponses: []*QueryRun{
				ready,
				running,
				running,
				failed,
			},
			want:       failed,
			wantTrials: 4,
		},
		{
			name: "cancelled",
			apiResponses: []*QueryRun{
				ready,
				cancelled,
			},
			want:       cancelled,
			wantTrials: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			resp := QueryRunResponses{responses: tt.apiResponses}
			server := newFlipsideMockServer(t, resp.next)

			fs := &Config{APIKey: "test", APIURL: server.URL}

			got, err := fs.AwaitQueryRunExecution(ctx, "", 10*time.Millisecond, 2)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetQueryRunResults() err: %v, wantErr: %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetQueryRunResults() diff (+got -want): %s", diff)
			}

			if resp.numCalls != tt.wantTrials {
				t.Errorf("GetQueryRunResults() numCalls: got=%d, want=%d", resp.numCalls, tt.wantTrials)
			}
		})
	}
}

type getQueryRunResultsTest[T any] struct {
	name        string
	apiResponse string
	want        *QueryRunResults[T]
	wantErr     error
}

func run[T any](t *testing.T, tests []getQueryRunResultsTest[T]) {
	t.Helper()

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			server := newFlipsideMockServer(t, func() string { return tt.apiResponse })

			fs := &Config{APIKey: "test", APIURL: server.URL}

			got, err := GetQueryRunResults[T](ctx, fs, "", 0)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GetQueryRunResults() err: %v, wantErr: %v", err, tt.wantErr)
			}

			if diff := cmp.Diff(tt.want, got); tt.want != nil && diff != "" {
				t.Errorf("GetQueryRunResults() diff (+got -want): %s", diff)
			}
		})
	}
}

func TestGetQueryRunResults(t *testing.T) {
	type MyResults struct {
		HourlyDatetime time.Time `json:"hourly_datetime"`
		TxCount        int       `json:"tx_count"`
		RowIndex       int       `json:"__row_index"`
	}

	run(t, []getQueryRunResultsTest[MyResults]{
		{
			name: "ok",
			// generated via postman https://api-docs.flipsidecrypto.xyz/#e5abf517-fba6-4fa8-81a5-dbcb1fb719e5
			apiResponse: `{"jsonrpc":"2.0","id":1,"result":{"columnNames":["hourly_datetime","tx_count","__row_index"],"columnTypes":["date","number","number"],"rows":[{"hourly_datetime":"2023-08-25T12:00:00.000Z","tx_count":36583,"__row_index":0},{"hourly_datetime":"2023-08-25T11:00:00.000Z","tx_count":45288,"__row_index":1},{"hourly_datetime":"2023-08-25T10:00:00.000Z","tx_count":42640,"__row_index":2},{"hourly_datetime":"2023-08-25T09:00:00.000Z","tx_count":46021,"__row_index":3},{"hourly_datetime":"2023-08-25T08:00:00.000Z","tx_count":44856,"__row_index":4}],"page":{"currentPageNumber":1,"currentPageSize":5,"totalRows":744,"totalPages":149},"sql":"select * from read_parquet('/data/2023/08/25/13/cllqm9bdr024hol0t1ccasfo2/*') offset 0 limit 5","format":"json","originalQueryRun":{"id":"cllqm54em023wol0trw4rsqfp","sqlStatementId":"clilbuyiu0g9hoo0tugawz6xa","state":"QUERY_STATE_SUCCESS","path":"2023/08/25/13/cllqm54em023wol0trw4rsqfp","fileCount":1,"lastFileNumber":null,"fileNames":"cllqm54em023wol0trw4rsqfp_results.parquet","errorName":null,"errorMessage":null,"errorData":null,"dataSourceQueryId":null,"dataSourceSessionId":"17257398909132306","startedAt":"2023-08-25T13:13:07.000Z","queryRunningEndedAt":"2023-08-25T13:13:17.000Z","queryStreamingEndedAt":"2023-08-25T13:13:17.000Z","endedAt":"2023-08-25T13:13:17.000Z","rowCount":744,"totalSize":"15293","tags":{"env":"test","source":"postman-demo"},"dataSourceId":"clf90gwee0002jvbu63diaa8u","userId":"clgzutzc9024fn90trhavr7xw","createdAt":"2023-08-25T13:13:06.000Z","updatedAt":"2023-08-25T13:13:06.000Z","archivedAt":"2023-08-25T13:16:36.000Z"},"redirectedToQueryRun":{"id":"cllqm9bdr024hol0t1ccasfo2","sqlStatementId":"clilbuyiu0g9hoo0tugawz6xa","state":"QUERY_STATE_SUCCESS","path":"2023/08/25/13/cllqm9bdr024hol0t1ccasfo2","fileCount":1,"lastFileNumber":null,"fileNames":"cllqm9bdr024hol0t1ccasfo2_results.parquet","errorName":null,"errorMessage":null,"errorData":null,"dataSourceQueryId":null,"dataSourceSessionId":"17257398909140842","startedAt":"2023-08-25T13:16:22.000Z","queryRunningEndedAt":"2023-08-25T13:16:33.000Z","queryStreamingEndedAt":"2023-08-25T13:16:33.000Z","endedAt":"2023-08-25T13:16:33.000Z","rowCount":744,"totalSize":"15293","tags":{"env":"test","source":"postman-demo"},"dataSourceId":"clf90gwee0002jvbu63diaa8u","userId":"clgzutzc9024fn90trhavr7xw","createdAt":"2023-08-25T13:16:22.000Z","updatedAt":"2023-08-25T13:16:22.000Z","archivedAt":null}}}`,
			want: &QueryRunResults[MyResults]{
				ColumnNames: []string{
					"hourly_datetime",
					"tx_count",
					"__row_index",
				},
				ColumnTypes: []string{
					"date",
					"number",
					"number",
				},
				Rows: []MyResults{
					{timeFrom("2023-08-25T12:00:00.000Z"), 36583, 0},
					{timeFrom("2023-08-25T11:00:00.000Z"), 45288, 1},
					{timeFrom("2023-08-25T10:00:00.000Z"), 42640, 2},
					{timeFrom("2023-08-25T09:00:00.000Z"), 46021, 3},
					{timeFrom("2023-08-25T08:00:00.000Z"), 44856, 4},
				},
				Page: ResultsPage{
					CurrentPageNumber: 1,
					CurrentPageSize:   5,
					TotalRows:         744,
					TotalPages:        149,
				},
				SQL:    "select * from read_parquet('/data/2023/08/25/13/cllqm9bdr024hol0t1ccasfo2/*') offset 0 limit 5",
				Format: "json",
				OriginalQueryRun: QueryRun{
					ID:                    "cllqm54em023wol0trw4rsqfp",
					SQLStatementID:        "clilbuyiu0g9hoo0tugawz6xa",
					State:                 "QUERY_STATE_SUCCESS",
					Path:                  "2023/08/25/13/cllqm54em023wol0trw4rsqfp",
					FileCount:             1,
					LastFileNumber:        nil,
					FileNames:             "cllqm54em023wol0trw4rsqfp_results.parquet",
					ErrorName:             "",
					ErrorMessage:          nil,
					ErrorData:             nil,
					DataSourceQueryID:     "",
					DataSourceSessionID:   "17257398909132306",
					StartedAt:             timeFrom("2023-08-25T13:13:07.000Z"),
					QueryRunningEndedAt:   timeFrom("2023-08-25T13:13:17.000Z"),
					QueryStreamingEndedAt: timeFrom("2023-08-25T13:13:17.000Z"),
					EndedAt:               timeFrom("2023-08-25T13:13:17.000Z"),
					RowCount:              744,
					TotalSize:             "15293",
					Tags:                  Tags{"env": "test", "source": "postman-demo"},
					DataSourceID:          "clf90gwee0002jvbu63diaa8u",
					UserID:                "clgzutzc9024fn90trhavr7xw",
					CreatedAt:             timeFrom("2023-08-25T13:13:06.000Z"),
					UpdatedAt:             timeFrom("2023-08-25T13:13:06.000Z"),
					ArchivedAt:            timeFrom("2023-08-25T13:16:36.000Z"),
				},
				RedirectedToQueryRun: &QueryRun{
					ID:                    "cllqm9bdr024hol0t1ccasfo2",
					SQLStatementID:        "clilbuyiu0g9hoo0tugawz6xa",
					State:                 "QUERY_STATE_SUCCESS",
					Path:                  "2023/08/25/13/cllqm9bdr024hol0t1ccasfo2",
					FileCount:             1,
					LastFileNumber:        nil,
					FileNames:             "cllqm9bdr024hol0t1ccasfo2_results.parquet",
					ErrorName:             "",
					ErrorMessage:          nil,
					ErrorData:             nil,
					DataSourceQueryID:     "",
					DataSourceSessionID:   "17257398909140842",
					StartedAt:             timeFrom("2023-08-25T13:16:22.000Z"),
					QueryRunningEndedAt:   timeFrom("2023-08-25T13:16:33.000Z"),
					QueryStreamingEndedAt: timeFrom("2023-08-25T13:16:33.000Z"),
					EndedAt:               timeFrom("2023-08-25T13:16:33.000Z"),
					RowCount:              744,
					TotalSize:             "15293",
					Tags:                  Tags{"env": "test", "source": "postman-demo"},
					DataSourceID:          "clf90gwee0002jvbu63diaa8u",
					UserID:                "clgzutzc9024fn90trhavr7xw",
					CreatedAt:             timeFrom("2023-08-25T13:16:22.000Z"),
					UpdatedAt:             timeFrom("2023-08-25T13:16:22.000Z"),
					ArchivedAt:            time.Time{},
				},
			},
		},
	})
}
