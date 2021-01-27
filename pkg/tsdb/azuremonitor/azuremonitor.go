package azuremonitor

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/setting"
	"github.com/grafana/grafana/pkg/tsdb"
)

var (
	azlog           log.Logger
	legendKeyFormat *regexp.Regexp
)

// azureMonitorExecutor executes queries for the Azure Monitor datasource - all four services
type azureMonitorExecutor struct {
	httpClient *http.Client
	dsInfo     *models.DataSource
}

// newAzureMonitorExecutor initializes a http client
func newAzureMonitorExecutor(dsInfo *models.DataSource, cfg *setting.Cfg) (tsdb.TsdbQueryEndpoint, error) {
	httpClient, err := dsInfo.GetHttpClient()
	if err != nil {
		return nil, err
	}

	return &azureMonitorExecutor{
		httpClient: httpClient,
		dsInfo:     dsInfo,
	}, nil
}

func init() {
	azlog = log.New("tsdb.azuremonitor")
	tsdb.RegisterTSDBQueryEndpoint("grafana-azure-monitor-datasource", newAzureMonitorExecutor)
	legendKeyFormat = regexp.MustCompile(`\{\{\s*(.+?)\s*\}\}`)
}

// Query takes in the frontend queries, parses them into the query format
// expected by chosen Azure Monitor service (Azure Monitor, App Insights etc.)
// executes the queries against the API and parses the response into
// the right format
func (e *azureMonitorExecutor) Query(ctx context.Context, dsInfo *models.DataSource, tsdbQuery *tsdb.TsdbQuery) (*tsdb.Response, error) {
	var err error

	var azureMonitorQueries []*tsdb.Query
	var applicationInsightsQueries []*tsdb.Query
	var azureLogAnalyticsQueries []*tsdb.Query
	var insightsAnalyticsQueries []*tsdb.Query

	for _, query := range tsdbQuery.Queries {
		queryType := query.Model.Get("queryType").MustString("")

		switch queryType {
		case "Azure Monitor":
			azureMonitorQueries = append(azureMonitorQueries, query)
		case "Application Insights":
			applicationInsightsQueries = append(applicationInsightsQueries, query)
		case "Azure Log Analytics":
			azureLogAnalyticsQueries = append(azureLogAnalyticsQueries, query)
		case "Insights Analytics":
			insightsAnalyticsQueries = append(insightsAnalyticsQueries, query)
		default:
			return nil, fmt.Errorf("alerting not supported for %q", queryType)
		}
	}

	azDatasource := &AzureMonitorDatasource{
		httpClient: e.httpClient,
		dsInfo:     e.dsInfo,
	}

	aiDatasource := &ApplicationInsightsDatasource{
		httpClient: e.httpClient,
		dsInfo:     e.dsInfo,
	}

	alaDatasource := &AzureLogAnalyticsDatasource{
		httpClient: e.httpClient,
		dsInfo:     e.dsInfo,
	}

	iaDatasource := &InsightsAnalyticsDatasource{
		httpClient: e.httpClient,
		dsInfo:     e.dsInfo,
	}

	azResult, err := azDatasource.executeTimeSeriesQuery(ctx, azureMonitorQueries, tsdbQuery.TimeRange)
	if err != nil {
		return nil, err
	}

	aiResult, err := aiDatasource.executeTimeSeriesQuery(ctx, applicationInsightsQueries, tsdbQuery.TimeRange)
	if err != nil {
		return nil, err
	}

	alaResult, err := alaDatasource.executeTimeSeriesQuery(ctx, azureLogAnalyticsQueries, tsdbQuery.TimeRange)
	if err != nil {
		return nil, err
	}

	iaResult, err := iaDatasource.executeTimeSeriesQuery(ctx, insightsAnalyticsQueries, tsdbQuery.TimeRange)
	if err != nil {
		return nil, err
	}

	for k, v := range aiResult.Results {
		azResult.Results[k] = v
	}

	for k, v := range alaResult.Results {
		azResult.Results[k] = v
	}

	for k, v := range iaResult.Results {
		azResult.Results[k] = v
	}

	return azResult, nil
}
