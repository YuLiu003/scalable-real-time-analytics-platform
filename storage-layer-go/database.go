package main

import (
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
)

// DB represents the database connection and operations
type DB struct {
	db    *sql.DB
	mutex sync.Mutex // For thread safety
}

// NewDB creates a new database connection
func NewDB(dbPath string) (*DB, error) {
	// Ensure data directory exists
	ensureDataDir(dbPath)

	// Open database connection
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db: db}, nil
}

// InitSchema initializes the database schema
func (d *DB) InitSchema() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Create sensor_data table
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS sensor_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id TEXT NOT NULL,
			temperature REAL,
			humidity REAL,
			timestamp REAL NOT NULL,
			is_anomaly INTEGER,
			anomaly_score REAL,
			tenant_id TEXT NOT NULL,
			created_at REAL NOT NULL
		)
	`)
	if err != nil {
		return err
	}

	// Create indexes for efficient queries
	_, err = d.db.Exec("CREATE INDEX IF NOT EXISTS idx_tenant_id ON sensor_data (tenant_id)")
	if err != nil {
		return err
	}

	_, err = d.db.Exec("CREATE INDEX IF NOT EXISTS idx_device_id ON sensor_data (device_id)")
	if err != nil {
		return err
	}

	log.Println("Database schema initialized with tenant isolation")
	return nil
}

// StoreSensorData stores sensor data in the database
func (d *DB) StoreSensorData(data *SensorDataRequest) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Start timer for metrics
	startTime := time.Now()

	// Prepare query
	stmt, err := d.db.Prepare(`
		INSERT INTO sensor_data 
		(device_id, temperature, humidity, timestamp, is_anomaly, anomaly_score, tenant_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		storageCount.With(prometheus.Labels{"operation": "insert", "status": "error"}).Inc()
		return err
	}
	defer stmt.Close()

	// Default values for optional fields
	isAnomaly := 0
	if data.IsAnomaly != nil && *data.IsAnomaly {
		isAnomaly = 1
	}

	// Execute query
	_, err = stmt.Exec(
		data.DeviceID,
		data.Temperature,
		data.Humidity,
		data.Timestamp.Unix(),
		isAnomaly,
		data.AnomalyScore,
		data.TenantID,
		time.Now().Unix(),
	)
	if err != nil {
		storageCount.With(prometheus.Labels{"operation": "insert", "status": "error"}).Inc()
		return err
	}

	// Update metrics
	storageCount.With(prometheus.Labels{"operation": "insert", "status": "success"}).Inc()
	storageLatency.With(prometheus.Labels{"operation": "insert"}).Observe(time.Since(startTime).Seconds())

	// Update record count gauge
	go d.UpdateRecordCounts()

	return nil
}

// GetTenantData retrieves data for a specific tenant with pagination
func (d *DB) GetTenantData(tenantID string, limit, offset int) ([]SensorData, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Start timer for metrics
	startTime := time.Now()

	// Prepare query
	query := `
		SELECT id, device_id, temperature, humidity, timestamp, is_anomaly, anomaly_score, tenant_id, created_at
		FROM sensor_data
		WHERE tenant_id = ?
		ORDER BY timestamp DESC
		LIMIT ? OFFSET ?
	`
	rows, err := d.db.Query(query, tenantID, limit, offset)
	if err != nil {
		storageCount.With(prometheus.Labels{"operation": "query", "status": "error"}).Inc()
		return nil, err
	}
	defer rows.Close()

	// Parse results
	results := make([]SensorData, 0) // Initialize as empty slice instead of nil
	for rows.Next() {
		var data SensorData
		var timestamp, createdAt float64
		var temp, humidity, anomalyScore sql.NullFloat64
		var isAnomaly int

		err := rows.Scan(
			&data.ID,
			&data.DeviceID,
			&temp,
			&humidity,
			&timestamp,
			&isAnomaly,
			&anomalyScore,
			&data.TenantID,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullables to pointers
		if temp.Valid {
			data.Temperature = &temp.Float64
		}
		if humidity.Valid {
			data.Humidity = &humidity.Float64
		}
		if anomalyScore.Valid {
			data.AnomalyScore = &anomalyScore.Float64
		}

		// Convert timestamps from float64 to time.Time
		data.Timestamp = time.Unix(int64(timestamp), 0)
		data.CreatedAt = time.Unix(int64(createdAt), 0)
		data.IsAnomaly = isAnomaly == 1

		results = append(results, data)
	}

	// Update metrics
	storageCount.With(prometheus.Labels{"operation": "query", "status": "success"}).Inc()
	storageLatency.With(prometheus.Labels{"operation": "query"}).Observe(time.Since(startTime).Seconds())

	return results, nil
}

// GetTenantDeviceData retrieves data for a specific device of a tenant
func (d *DB) GetTenantDeviceData(tenantID, deviceID string, limit int) ([]SensorData, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Start timer for metrics
	startTime := time.Now()

	// Prepare query
	query := `
		SELECT id, device_id, temperature, humidity, timestamp, is_anomaly, anomaly_score, tenant_id, created_at
		FROM sensor_data
		WHERE tenant_id = ? AND device_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`
	rows, err := d.db.Query(query, tenantID, deviceID, limit)
	if err != nil {
		storageCount.With(prometheus.Labels{"operation": "query_device", "status": "error"}).Inc()
		return nil, err
	}
	defer rows.Close()

	// Parse results
	results := make([]SensorData, 0) // Initialize as empty slice instead of nil
	for rows.Next() {
		var data SensorData
		var timestamp, createdAt float64
		var temp, humidity, anomalyScore sql.NullFloat64
		var isAnomaly int

		err := rows.Scan(
			&data.ID,
			&data.DeviceID,
			&temp,
			&humidity,
			&timestamp,
			&isAnomaly,
			&anomalyScore,
			&data.TenantID,
			&createdAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert nullables to pointers
		if temp.Valid {
			data.Temperature = &temp.Float64
		}
		if humidity.Valid {
			data.Humidity = &humidity.Float64
		}
		if anomalyScore.Valid {
			data.AnomalyScore = &anomalyScore.Float64
		}

		// Convert timestamps from float64 to time.Time
		data.Timestamp = time.Unix(int64(timestamp), 0)
		data.CreatedAt = time.Unix(int64(createdAt), 0)
		data.IsAnomaly = isAnomaly == 1

		results = append(results, data)
	}

	// Update metrics
	storageCount.With(prometheus.Labels{"operation": "query_device", "status": "success"}).Inc()
	storageLatency.With(prometheus.Labels{"operation": "query_device"}).Observe(time.Since(startTime).Seconds())

	return results, nil
}

// GetTenantStats calculates statistics for a tenant
func (d *DB) GetTenantStats(tenantID string) (*TenantStats, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Start timer for metrics
	startTime := time.Now()

	// Prepare query
	query := `
		SELECT 
			COUNT(DISTINCT device_id) as device_count,
			COUNT(*) as total_records,
			SUM(is_anomaly) as anomaly_count,
			AVG(temperature) as avg_temperature,
			AVG(humidity) as avg_humidity
		FROM sensor_data
		WHERE tenant_id = ?
	`
	var stats TenantStats
	var avgTemp, avgHumidity sql.NullFloat64

	err := d.db.QueryRow(query, tenantID).Scan(
		&stats.DeviceCount,
		&stats.TotalRecords,
		&stats.AnomalyCount,
		&avgTemp,
		&avgHumidity,
	)
	if err != nil {
		storageCount.With(prometheus.Labels{"operation": "stats", "status": "error"}).Inc()
		return nil, err
	}

	// Convert nullables
	if avgTemp.Valid {
		stats.AvgTemperature = avgTemp.Float64
	}
	if avgHumidity.Valid {
		stats.AvgHumidity = avgHumidity.Float64
	}

	stats.LastUpdated = time.Now().Format(time.RFC3339)

	// Update metrics
	storageCount.With(prometheus.Labels{"operation": "stats", "status": "success"}).Inc()
	storageLatency.With(prometheus.Labels{"operation": "stats"}).Observe(time.Since(startTime).Seconds())

	return &stats, nil
}

// ApplyRetentionPolicies applies tenant-specific retention policies
func (d *DB) ApplyRetentionPolicies(policies map[string]RetentionPolicy, defaultDays int) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	startTime := time.Now()
	log.Println("Applying tenant retention policies")

	// Begin transaction
	tx, err := d.db.Begin()
	if err != nil {
		storageCount.With(prometheus.Labels{"operation": "retention", "status": "error"}).Inc()
		return err
	}
	defer tx.Rollback()

	// Apply retention policy for each tenant
	for tenantID, policy := range policies {
		retentionDays := policy.Days
		maxAge := time.Now().AddDate(0, 0, -retentionDays).Unix()

		result, err := tx.Exec(`
			DELETE FROM sensor_data 
			WHERE tenant_id = ? AND created_at < ?
			`, tenantID, maxAge)
		if err != nil {
			storageCount.With(prometheus.Labels{"operation": "retention", "status": "error"}).Inc()
			return err
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("Applied %d day retention policy for tenant %s: %d records deleted",
			retentionDays, tenantID, rowsAffected)
	}

	// Apply default retention policy for tenants not explicitly defined
	defaultMaxAge := time.Now().AddDate(0, 0, -defaultDays).Unix()

	// Get list of tenants with explicit policies
	var tenantsList []string
	for tenantID := range policies {
		tenantsList = append(tenantsList, tenantID)
	}

	// Apply default policy to other tenants
	if len(tenantsList) > 0 {
		// Prepare NOT IN query with placeholders
		// Build placeholders safely using strings.Builder
		var queryBuilder strings.Builder
		args := make([]interface{}, 0, len(tenantsList)+1)

		queryBuilder.WriteString("DELETE FROM sensor_data WHERE tenant_id NOT IN (")
		for i, tenantID := range tenantsList {
			if i > 0 {
				queryBuilder.WriteString(",")
			}
			queryBuilder.WriteString("?")
			args = append(args, tenantID)
		}
		queryBuilder.WriteString(") AND created_at < ?")
		args = append(args, defaultMaxAge)

		// Execute query with safely built query string
		result, err := tx.Exec(queryBuilder.String(), args...)
		if err != nil {
			storageCount.With(prometheus.Labels{"operation": "retention", "status": "error"}).Inc()
			return err
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("Applied default %d day retention policy: %d records deleted", defaultDays, rowsAffected)
	} else {
		// No tenant-specific policies, apply default to all
		result, err := tx.Exec(`
			DELETE FROM sensor_data 
			WHERE created_at < ?
			`, defaultMaxAge)
		if err != nil {
			storageCount.With(prometheus.Labels{"operation": "retention", "status": "error"}).Inc()
			return err
		}

		rowsAffected, _ := result.RowsAffected()
		log.Printf("Applied default %d day retention policy: %d records deleted", defaultDays, rowsAffected)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		storageCount.With(prometheus.Labels{"operation": "retention", "status": "error"}).Inc()
		return err
	}

	// Update metrics
	storageCount.With(prometheus.Labels{"operation": "retention", "status": "success"}).Inc()
	storageLatency.With(prometheus.Labels{"operation": "retention"}).Observe(time.Since(startTime).Seconds())

	// Update record count gauge
	go d.UpdateRecordCounts()

	return nil
}

// UpdateRecordCounts updates the metrics for records per tenant
func (d *DB) UpdateRecordCounts() {
	// Create a new lock so this doesn't block other operations
	var mutex sync.Mutex
	mutex.Lock()
	defer mutex.Unlock()

	query := `SELECT tenant_id, COUNT(*) FROM sensor_data GROUP BY tenant_id`
	rows, err := d.db.Query(query)
	if err != nil {
		log.Printf("Error updating record counts: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var tenantID string
		var count int
		if err := rows.Scan(&tenantID, &count); err != nil {
			log.Printf("Error scanning record count: %v", err)
			continue
		}
		recordsByTenant.With(prometheus.Labels{"tenant_id": tenantID}).Set(float64(count))
	}
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.db.Close()
}
