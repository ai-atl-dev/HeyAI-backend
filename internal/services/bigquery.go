package services

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/ai-atl-dev/HeyAI-backend/internal/models"
	"google.golang.org/api/iterator"
)

// BigQueryService handles BigQuery operations
type BigQueryService struct {
	client        *bigquery.Client
	datasetID     string
	callsTable    string
	usageTable    string
	paymentsTable string
}

// NewBigQueryService creates a new BigQuery service
func NewBigQueryService(ctx context.Context, config *models.Config) (*BigQueryService, error) {
	client, err := bigquery.NewClient(ctx, config.GCPProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create BigQuery client: %w", err)
	}

	return &BigQueryService{
		client:        client,
		datasetID:     config.BigQueryDataset,
		callsTable:    config.BigQueryCallsTable,
		usageTable:    config.BigQueryUsageTable,
		paymentsTable: config.BigQueryPaymentsTable,
	}, nil
}

// Close closes the BigQuery client
func (s *BigQueryService) Close() error {
	return s.client.Close()
}

// Call Operations

// InsertCall inserts a call record into BigQuery
func (s *BigQueryService) InsertCall(ctx context.Context, call *models.Call) error {
	inserter := s.client.Dataset(s.datasetID).Table(s.callsTable).Inserter()

	if err := inserter.Put(ctx, call); err != nil {
		return fmt.Errorf("failed to insert call: %w", err)
	}

	return nil
}

// InsertCalls inserts multiple call records
func (s *BigQueryService) InsertCalls(ctx context.Context, calls []*models.Call) error {
	inserter := s.client.Dataset(s.datasetID).Table(s.callsTable).Inserter()

	if err := inserter.Put(ctx, calls); err != nil {
		return fmt.Errorf("failed to insert calls: %w", err)
	}

	return nil
}

// GetCallsByAgent retrieves calls for a specific agent
func (s *BigQueryService) GetCallsByAgent(ctx context.Context, agentID string, limit int) ([]*models.Call, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT *
		FROM %s.%s.%s
		WHERE agent_id = @agentID
		ORDER BY start_time DESC
		LIMIT @limit
	`, s.client.Project(), s.datasetID, s.callsTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "agentID", Value: agentID},
		{Name: "limit", Value: limit},
	}

	return s.executeCallQuery(ctx, query)
}

// GetCallsByDateRange retrieves calls within a date range
func (s *BigQueryService) GetCallsByDateRange(ctx context.Context, startDate, endDate time.Time) ([]*models.Call, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT *
		FROM %s.%s.%s
		WHERE start_time >= @startDate AND start_time <= @endDate
		ORDER BY start_time DESC
	`, s.client.Project(), s.datasetID, s.callsTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "startDate", Value: startDate},
		{Name: "endDate", Value: endDate},
	}

	return s.executeCallQuery(ctx, query)
}

// GetCallStats retrieves call statistics
func (s *BigQueryService) GetCallStats(ctx context.Context, agentID string, startDate, endDate time.Time) (map[string]interface{}, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT
			COUNT(*) as total_calls,
			SUM(duration) as total_duration,
			AVG(duration) as avg_duration,
			SUM(cost) as total_cost,
			COUNT(DISTINCT caller_number) as unique_callers
		FROM %s.%s.%s
		WHERE agent_id = @agentID
		AND start_time >= @startDate
		AND start_time <= @endDate
	`, s.client.Project(), s.datasetID, s.callsTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "agentID", Value: agentID},
		{Name: "startDate", Value: startDate},
		{Name: "endDate", Value: endDate},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var row map[string]bigquery.Value
	err = it.Next(&row)
	if err != nil {
		return nil, fmt.Errorf("failed to read row: %w", err)
	}

	stats := make(map[string]interface{})
	for k, v := range row {
		stats[k] = v
	}

	return stats, nil
}

// Usage History Operations

// InsertUsageHistory inserts usage history record
func (s *BigQueryService) InsertUsageHistory(ctx context.Context, usage *models.UsageHistory) error {
	inserter := s.client.Dataset(s.datasetID).Table(s.usageTable).Inserter()

	if err := inserter.Put(ctx, usage); err != nil {
		return fmt.Errorf("failed to insert usage history: %w", err)
	}

	return nil
}

// GetUsageHistory retrieves usage history for a user
func (s *BigQueryService) GetUsageHistory(ctx context.Context, userID string, limit int) ([]*models.UsageHistory, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT *
		FROM %s.%s.%s
		WHERE user_id = @userID
		ORDER BY date DESC
		LIMIT @limit
	`, s.client.Project(), s.datasetID, s.usageTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "userID", Value: userID},
		{Name: "limit", Value: limit},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var usageHistory []*models.UsageHistory
	for {
		var usage models.UsageHistory
		err := it.Next(&usage)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read row: %w", err)
		}
		usageHistory = append(usageHistory, &usage)
	}

	return usageHistory, nil
}

// GetDailyUsage retrieves daily usage for an agent
func (s *BigQueryService) GetDailyUsage(ctx context.Context, agentID string, date time.Time) (*models.UsageHistory, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT
			@agentID as agent_id,
			@date as date,
			COUNT(*) as total_calls,
			SUM(duration) as total_duration,
			SUM(cost) as total_cost
		FROM %s.%s.%s
		WHERE agent_id = @agentID
		AND DATE(start_time) = DATE(@date)
	`, s.client.Project(), s.datasetID, s.callsTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "agentID", Value: agentID},
		{Name: "date", Value: date},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var usage models.UsageHistory
	err = it.Next(&usage)
	if err != nil {
		return nil, fmt.Errorf("failed to read row: %w", err)
	}

	return &usage, nil
}

// Payment Operations

// InsertPayment inserts a payment record
func (s *BigQueryService) InsertPayment(ctx context.Context, payment *models.Payment) error {
	inserter := s.client.Dataset(s.datasetID).Table(s.paymentsTable).Inserter()

	if err := inserter.Put(ctx, payment); err != nil {
		return fmt.Errorf("failed to insert payment: %w", err)
	}

	return nil
}

// GetPaymentsByUser retrieves payments for a user
func (s *BigQueryService) GetPaymentsByUser(ctx context.Context, userID string, limit int) ([]*models.Payment, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT *
		FROM %s.%s.%s
		WHERE user_id = @userID
		ORDER BY created_at DESC
		LIMIT @limit
	`, s.client.Project(), s.datasetID, s.paymentsTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "userID", Value: userID},
		{Name: "limit", Value: limit},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var payments []*models.Payment
	for {
		var payment models.Payment
		err := it.Next(&payment)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read row: %w", err)
		}
		payments = append(payments, &payment)
	}

	return payments, nil
}

// Analytics Operations

// GetTopCallers retrieves top callers by call count
func (s *BigQueryService) GetTopCallers(ctx context.Context, agentID string, limit int) ([]map[string]interface{}, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT
			caller_number,
			COUNT(*) as call_count,
			SUM(duration) as total_duration,
			AVG(duration) as avg_duration
		FROM %s.%s.%s
		WHERE agent_id = @agentID
		GROUP BY caller_number
		ORDER BY call_count DESC
		LIMIT @limit
	`, s.client.Project(), s.datasetID, s.callsTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "agentID", Value: agentID},
		{Name: "limit", Value: limit},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var results []map[string]interface{}
	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read row: %w", err)
		}

		result := make(map[string]interface{})
		for k, v := range row {
			result[k] = v
		}
		results = append(results, result)
	}

	return results, nil
}

// GetCallTrends retrieves call trends over time
func (s *BigQueryService) GetCallTrends(ctx context.Context, agentID string, days int) ([]map[string]interface{}, error) {
	query := s.client.Query(fmt.Sprintf(`
		SELECT
			DATE(start_time) as date,
			COUNT(*) as call_count,
			SUM(duration) as total_duration,
			AVG(duration) as avg_duration,
			SUM(cost) as total_cost
		FROM %s.%s.%s
		WHERE agent_id = @agentID
		AND start_time >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL @days DAY)
		GROUP BY date
		ORDER BY date DESC
	`, s.client.Project(), s.datasetID, s.callsTable))

	query.Parameters = []bigquery.QueryParameter{
		{Name: "agentID", Value: agentID},
		{Name: "days", Value: days},
	}

	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var results []map[string]interface{}
	for {
		var row map[string]bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read row: %w", err)
		}

		result := make(map[string]interface{})
		for k, v := range row {
			result[k] = v
		}
		results = append(results, result)
	}

	return results, nil
}

// Helper function to execute call queries
func (s *BigQueryService) executeCallQuery(ctx context.Context, query *bigquery.Query) ([]*models.Call, error) {
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	var calls []*models.Call
	for {
		var call models.Call
		err := it.Next(&call)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read row: %w", err)
		}
		calls = append(calls, &call)
	}

	return calls, nil
}

// CreateTables creates BigQuery tables if they don't exist
func (s *BigQueryService) CreateTables(ctx context.Context) error {
	dataset := s.client.Dataset(s.datasetID)

	// Create calls table
	callsTableRef := dataset.Table(s.callsTable)
	callsSchema, err := bigquery.InferSchema(models.Call{})
	if err != nil {
		return fmt.Errorf("failed to infer calls schema: %w", err)
	}

	if err := callsTableRef.Create(ctx, &bigquery.TableMetadata{Schema: callsSchema}); err != nil {
		// Table might already exist, ignore error
		fmt.Printf("Calls table creation: %v\n", err)
	}

	// Create usage table
	usageTableRef := dataset.Table(s.usageTable)
	usageSchema, err := bigquery.InferSchema(models.UsageHistory{})
	if err != nil {
		return fmt.Errorf("failed to infer usage schema: %w", err)
	}

	if err := usageTableRef.Create(ctx, &bigquery.TableMetadata{Schema: usageSchema}); err != nil {
		fmt.Printf("Usage table creation: %v\n", err)
	}

	// Create payments table
	paymentsTableRef := dataset.Table(s.paymentsTable)
	paymentsSchema, err := bigquery.InferSchema(models.Payment{})
	if err != nil {
		return fmt.Errorf("failed to infer payments schema: %w", err)
	}

	if err := paymentsTableRef.Create(ctx, &bigquery.TableMetadata{Schema: paymentsSchema}); err != nil {
		fmt.Printf("Payments table creation: %v\n", err)
	}

	return nil
}
