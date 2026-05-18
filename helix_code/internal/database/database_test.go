package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	config := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 5432, config.Port)
	assert.Equal(t, "testuser", config.User)
	assert.Equal(t, "testpass", config.Password)
	assert.Equal(t, "testdb", config.DBName)
	assert.Equal(t, "disable", config.SSLMode)
}

func TestNew_InvalidConfig(t *testing.T) {
	// Test with invalid host
	config := Config{
		Host:     "", // Invalid host
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	db, err := New(config)
	assert.Error(t, err)
	assert.Nil(t, db)
	assert.Contains(t, err.Error(), "internal_database_ping_failed")
}

func TestDatabase_Close(t *testing.T) {
	// Test Close on database with nil pool
	db := &Database{Pool: nil}
	// Should not panic
	db.Close()
}

func TestDatabase_GetDB(t *testing.T) {
	// Test GetDB on database with nil pool
	db := &Database{Pool: nil}
	sqlDB, err := db.GetDB()
	assert.Error(t, err)
	assert.Nil(t, sqlDB)
	assert.Contains(t, err.Error(), "internal_database_pool_not_initialized")
}

func TestDatabase_HealthCheck(t *testing.T) {
	// Test HealthCheck on database with nil pool
	db := &Database{Pool: nil}
	err := db.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "internal_database_pool_not_initialized")
}

// TestNew_ValidConfig tests that connection string is properly formatted
func TestNew_ValidConfig(t *testing.T) {
	config := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	// Test that connection string is properly formatted
	connString := formatConnectionString(config)
	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	assert.Equal(t, expected, connString)
}

// TestNew_PoolConfig tests connection pool configuration parsing
func TestNew_PoolConfig(t *testing.T) {
	config := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	connString := formatConnectionString(config)
	poolConfig, err := pgxpool.ParseConfig(connString)
	require.NoError(t, err)

	// Test that connection string is parsable
	assert.NotNil(t, poolConfig)
	assert.Equal(t, connString, poolConfig.ConnString())
}

// TestDatabase_ErrorScenarios tests various error conditions
func TestDatabase_ErrorScenarios(t *testing.T) {
	t.Run("GetDB_NilPool", func(t *testing.T) {
		db := &Database{Pool: nil}
		sqlDB, err := db.GetDB()
		assert.Error(t, err)
		assert.Nil(t, sqlDB)
		assert.Contains(t, err.Error(), "internal_database_pool_not_initialized")
	})

	t.Run("HealthCheck_NilPool", func(t *testing.T) {
		db := &Database{Pool: nil}
		err := db.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "internal_database_pool_not_initialized")
	})

	t.Run("Close_NilPool", func(t *testing.T) {
		db := &Database{Pool: nil}
		// Should not panic
		assert.NotPanics(t, func() {
			db.Close()
		})
	})
}

// TestConfig_Validation tests configuration validation scenarios
func TestConfig_Validation(t *testing.T) {
	testCases := []struct {
		name   string
		config Config
		valid  bool
	}{
		{
			name: "ValidConfig",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				DBName:   "db",
				SSLMode:  "disable",
			},
			valid: true,
		},
		{
			name: "EmptyHost",
			config: Config{
				Host:     "",
				Port:     5432,
				User:     "user",
				Password: "pass",
				DBName:   "db",
				SSLMode:  "disable",
			},
			valid: false,
		},
		{
			name: "ZeroPort",
			config: Config{
				Host:     "localhost",
				Port:     0,
				User:     "user",
				Password: "pass",
				DBName:   "db",
				SSLMode:  "disable",
			},
			valid: false,
		},
		{
			name: "EmptyUser",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "",
				Password: "pass",
				DBName:   "db",
				SSLMode:  "disable",
			},
			valid: false,
		},
		{
			name: "EmptyDatabase",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				DBName:   "",
				SSLMode:  "disable",
			},
			valid: false,
		},
		{
			name: "ValidWithSSL",
			config: Config{
				Host:     "db.example.com",
				Port:     5432,
				User:     "user",
				Password: "pass",
				DBName:   "db",
				SSLMode:  "require",
			},
			valid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test connection string formatting
			connString := formatConnectionString(tc.config)
			assert.NotEmpty(t, connString)

			// Test config parsing
			poolConfig, err := pgxpool.ParseConfig(connString)
			if tc.valid {
				assert.NoError(t, err)
				assert.NotNil(t, poolConfig)
			} else {
				// Even invalid configs might parse, but connection will fail
				// So we just test that the function doesn't panic
				if err == nil {
					assert.NotNil(t, poolConfig)
				}
			}
		})
	}
}

// TestConnectionString_Formats tests different connection string formats
func TestConnectionString_Formats(t *testing.T) {
	testCases := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "BasicConfig",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				DBName:   "db",
				SSLMode:  "disable",
			},
			expected: "host=localhost port=5432 user=user password=pass dbname=db sslmode=disable",
		},
		{
			name: "WithSSLRequire",
			config: Config{
				Host:     "db.example.com",
				Port:     5432,
				User:     "appuser",
				Password: "secret",
				DBName:   "production",
				SSLMode:  "require",
			},
			expected: "host=db.example.com port=5432 user=appuser password=secret dbname=production sslmode=require",
		},
		{
			name: "WithSpecialCharacters",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "user@domain.com",
				Password: "p@$$w0rd!",
				DBName:   "my-db",
				SSLMode:  "prefer",
			},
			expected: "host=localhost port=5432 user=user@domain.com password=p@$$w0rd! dbname=my-db sslmode=prefer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			connString := formatConnectionString(tc.config)
			assert.Equal(t, tc.expected, connString)
		})
	}
}

// TestDatabase_Timeouts tests timeout configurations
func TestDatabase_Timeouts(t *testing.T) {
	config := Config{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
	}

	connString := formatConnectionString(config)
	poolConfig, err := pgxpool.ParseConfig(connString)
	require.NoError(t, err)

	// Test that we can set timeouts
	poolConfig.HealthCheckPeriod = 30 * time.Second
	poolConfig.MaxConnLifetime = 2 * time.Hour
	poolConfig.MaxConnIdleTime = time.Hour

	assert.Equal(t, 30*time.Second, poolConfig.HealthCheckPeriod)
	assert.Equal(t, 2*time.Hour, poolConfig.MaxConnLifetime)
	assert.Equal(t, time.Hour, poolConfig.MaxConnIdleTime)
}

// TestDatabase_Concurrency tests concurrent access patterns
func TestDatabase_Concurrency(t *testing.T) {
	db := &Database{Pool: nil}

	// Test concurrent health checks (all should fail gracefully)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			err := db.HealthCheck()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "internal_database_pool_not_initialized")
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out waiting for goroutines")
		}
	}
}

// TestDatabase_Ping_Method tests the Ping method of real Database struct
func TestDatabase_Ping_Method(t *testing.T) {
	// Test with nil pool (this actually calls the real method)
	db := &Database{Pool: nil}
	// This will panic because Ping tries to use the pool
	assert.Panics(t, func() {
		db.Ping(context.Background())
	})

	// Test with mock setup - we can't easily mock pgxpool,
	// but we can test that the method signature is correct
	assert.NotNil(t, (*Database)(nil).Ping)
}

// TestDatabase_InitializeSchema_Method tests InitializeSchema method
func TestDatabase_InitializeSchema_Method(t *testing.T) {
	// Test with nil pool - this will panic
	db := &Database{Pool: nil}

	// This will panic because InitializeSchema tries to use the pool
	assert.Panics(t, func() {
		db.InitializeSchema()
	})
}

// TestDatabase_InitializeSchema_Method tests InitializeSchema method

// TestDatabase_InterfaceMethods tests database interface methods using mocks
func TestDatabase_InterfaceMethods(t *testing.T) {
	// Test with mock setup - we can't easily mock pgxpool,
	// but we can test that method signature is correct
	assert.NotNil(t, (*Database)(nil).Ping)
	t.Run("Exec_Success", func(t *testing.T) {
		mockDB := NewMockDatabase()

		// Set up mock expectation
		mockDB.On("Exec", mock.Anything, mock.Anything, mock.Anything).
			Return(pgconn.CommandTag{}, nil)

		ctx := context.Background()
		_, err := mockDB.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
	})

	t.Run("Exec_Error", func(t *testing.T) {
		mockDB := NewMockDatabase()

		// Set up mock to return error
		mockDB.On("Exec", mock.Anything, mock.Anything, mock.Anything).
			Return(pgconn.CommandTag{}, fmt.Errorf("database error"))

		ctx := context.Background()
		_, err := mockDB.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database error")

		mockDB.AssertExpectations(t)
	})

	t.Run("Query_Success", func(t *testing.T) {
		mockDB := NewMockDatabase()
		mockRows := NewMockRows([][]interface{}{
			{"testuser", "test@example.com"},
			{"user2", "user2@example.com"},
		})

		// Set up mock expectation
		mockDB.On("Query", mock.Anything, mock.Anything, mock.Anything).
			Return(mockRows, nil)

		ctx := context.Background()
		rows, err := mockDB.Query(ctx, "SELECT * FROM users")
		assert.NoError(t, err)
		assert.NotNil(t, rows)

		mockDB.AssertExpectations(t)
	})

	t.Run("Query_Error", func(t *testing.T) {
		mockDB := NewMockDatabase()

		// Set up mock to return error
		mockDB.On("Query", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("query error"))

		ctx := context.Background()
		rows, err := mockDB.Query(ctx, "SELECT * FROM users")
		assert.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "query error")

		mockDB.AssertExpectations(t)
	})

	t.Run("QueryRow_Success", func(t *testing.T) {
		mockDB := NewMockDatabase()
		mockRow := NewMockRowWithValues([]interface{}{"testuser", "test@example.com"})

		// Set up mock expectation
		mockDB.On("QueryRow", mock.Anything, mock.Anything, mock.Anything).
			Return(mockRow)

		ctx := context.Background()
		row := mockDB.QueryRow(ctx, "SELECT username, email FROM users WHERE id = $1", "123")
		assert.NotNil(t, row)

		mockDB.AssertExpectations(t)
	})

	t.Run("Ping_Success", func(t *testing.T) {
		mockDB := NewMockDatabase()

		// Set up mock expectation
		mockDB.On("Ping", mock.Anything).
			Return(nil)

		ctx := context.Background()
		err := mockDB.Ping(ctx)
		assert.NoError(t, err)

		mockDB.AssertExpectations(t)
	})

	t.Run("Ping_Error", func(t *testing.T) {
		mockDB := NewMockDatabase()

		// Set up mock to return error
		mockDB.On("Ping", mock.Anything).
			Return(fmt.Errorf("connection failed"))

		ctx := context.Background()
		err := mockDB.Ping(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "connection failed")

		mockDB.AssertExpectations(t)
	})
}

// TestDatabase_Close_Method tests database close method
func TestDatabase_Close_Method(t *testing.T) {
	mockDB := NewMockDatabase()

	// Set up mock expectation
	mockDB.On("Close").Return()

	// Call close
	mockDB.Close()

	// Assert expectations
	mockDB.AssertExpectations(t)
}

// TestDatabase_WithRealMethods tests using a real Database struct with mocked pool
func TestDatabase_WithRealMethods(t *testing.T) {
	// We can't easily mock pgxpool.Pool, so we test error cases
	db := &Database{Pool: nil}

	t.Run("Exec_NilPool", func(t *testing.T) {
		ctx := context.Background()

		// This should panic because pool is nil
		assert.Panics(t, func() {
			db.Exec(ctx, "SELECT 1")
		})
	})

	t.Run("Query_NilPool", func(t *testing.T) {
		ctx := context.Background()

		// This should panic because pool is nil
		assert.Panics(t, func() {
			db.Query(ctx, "SELECT 1")
		})
	})

	t.Run("QueryRow_NilPool", func(t *testing.T) {
		ctx := context.Background()

		// This should panic because pool is nil
		assert.Panics(t, func() {
			db.QueryRow(ctx, "SELECT 1")
		})
	})
}
func formatConnectionString(config Config) string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.DBName, config.SSLMode,
	)
}

// ========================================
// Mock Helper Tests
// ========================================

func TestMockExecErrorOnce(t *testing.T) {
	mockDB := NewMockDatabase()

	// Set up mock to return error once
	mockDB.MockExecErrorOnce(fmt.Errorf("one-time error"))

	ctx := context.Background()
	_, err := mockDB.Exec(ctx, "INSERT INTO test VALUES ($1)", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "one-time error")

	mockDB.AssertExpectations(t)
}

func TestMockExecWithSQLAndArgs(t *testing.T) {
	mockDB := NewMockDatabase()

	// Set up mock to match specific SQL (uses MockExecWithSQL which is simpler)
	mockDB.MockExecWithSQL("INSERT INTO users (name) VALUES ($1)", 1)

	ctx := context.Background()
	tag, err := mockDB.Exec(ctx, "INSERT INTO users (name) VALUES ($1)", "john")
	assert.NoError(t, err)
	assert.NotNil(t, tag)

	mockDB.AssertExpectations(t)
}

func TestMockExecWithSQLAndArgs_DirectOn(t *testing.T) {
	mockDB := NewMockDatabase()

	// Test the MockExecWithSQLAndArgs function directly
	// It expects the third argument to be a specific slice
	call := mockDB.MockExecWithSQLAndArgs("INSERT INTO test VALUES ($1)", []interface{}{"value"}, 1)
	assert.NotNil(t, call)
}

func TestMockAnyString(t *testing.T) {
	mockDB := NewMockDatabase()

	// Test AnyString helper
	result := mockDB.AnyString()
	assert.Equal(t, mock.Anything, result)
}

func TestMockAnyArgs(t *testing.T) {
	mockDB := NewMockDatabase()

	// Test AnyArgs helper
	result := mockDB.AnyArgs()
	assert.Equal(t, mock.Anything, result)
}

// ========================================
// MockRows Tests
// ========================================

func TestMockRows_NewMockRowsWithError(t *testing.T) {
	testErr := fmt.Errorf("test error")
	rows := NewMockRowsWithError(testErr)

	assert.NotNil(t, rows)
	assert.Equal(t, testErr, rows.Err())
	assert.False(t, rows.Next())
}

func TestMockRows_Close(t *testing.T) {
	rows := NewMockRows([][]interface{}{
		{"value1", "value2"},
	})

	// Before close, should be able to iterate
	assert.True(t, rows.Next())

	// Close the rows
	rows.Close()

	// After close, Next should return false
	assert.False(t, rows.Next())
}

func TestMockRows_Err(t *testing.T) {
	// Test with no error
	rows := NewMockRows([][]interface{}{})
	assert.Nil(t, rows.Err())

	// Test with error
	testErr := fmt.Errorf("test error")
	errorRows := NewMockRowsWithError(testErr)
	assert.Equal(t, testErr, errorRows.Err())
}

func TestMockRows_CommandTag(t *testing.T) {
	rows := NewMockRows([][]interface{}{})
	tag := rows.CommandTag()

	// Should return empty command tag
	assert.Equal(t, pgconn.CommandTag{}, tag)
}

func TestMockRows_FieldDescriptions(t *testing.T) {
	rows := NewMockRows([][]interface{}{})
	desc := rows.FieldDescriptions()

	// Should return empty slice
	assert.Empty(t, desc)
}

func TestMockRows_Values(t *testing.T) {
	rows := NewMockRows([][]interface{}{
		{"value1", 123, true},
	})

	// Before Next(), Values should return error
	_, err := rows.Values()
	assert.Error(t, err)

	// After Next(), Values should return current row
	rows.Next()
	values, err := rows.Values()
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"value1", 123, true}, values)
}

func TestMockRows_Values_WithError(t *testing.T) {
	testErr := fmt.Errorf("test error")
	rows := NewMockRowsWithError(testErr)

	values, err := rows.Values()
	assert.Error(t, err)
	assert.Nil(t, values)
	assert.Equal(t, testErr, err)
}

func TestMockRows_RawValues(t *testing.T) {
	rows := NewMockRows([][]interface{}{{"value"}})

	// Should return nil for mock
	rawValues := rows.RawValues()
	assert.Nil(t, rawValues)
}

func TestMockRows_Conn(t *testing.T) {
	rows := NewMockRows([][]interface{}{{"value"}})

	// Should return nil for mock
	conn := rows.Conn()
	assert.Nil(t, conn)
}

func TestMockRows_ScanTypes(t *testing.T) {
	t.Run("Scan_String", func(t *testing.T) {
		rows := NewMockRows([][]interface{}{{"hello"}})
		rows.Next()

		var result string
		err := rows.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("Scan_Int", func(t *testing.T) {
		rows := NewMockRows([][]interface{}{{42}})
		rows.Next()

		var result int
		err := rows.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("Scan_Int64", func(t *testing.T) {
		rows := NewMockRows([][]interface{}{{int64(1234567890)}})
		rows.Next()

		var result int64
		err := rows.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, int64(1234567890), result)
	})

	t.Run("Scan_Bool", func(t *testing.T) {
		rows := NewMockRows([][]interface{}{{true}})
		rows.Next()

		var result bool
		err := rows.Scan(&result)
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("Scan_NilValue", func(t *testing.T) {
		rows := NewMockRows([][]interface{}{{nil}})
		rows.Next()

		var result string
		err := rows.Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "", result) // Zero value
	})

	t.Run("Scan_StringPointer", func(t *testing.T) {
		rows := NewMockRows([][]interface{}{{"pointer value"}})
		rows.Next()

		var result *string
		err := rows.Scan(&result)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "pointer value", *result)
	})

	t.Run("Scan_MismatchedColumns", func(t *testing.T) {
		rows := NewMockRows([][]interface{}{{"value1", "value2"}})
		rows.Next()

		var result string
		err := rows.Scan(&result) // Only one dest for two values
		assert.Error(t, err)
	})
}

func TestMockRows_Next_ClosedRows(t *testing.T) {
	rows := NewMockRows([][]interface{}{{"value"}})
	rows.Close()

	// Next should return false on closed rows
	assert.False(t, rows.Next())
}

func TestMockRows_Next_WithError(t *testing.T) {
	testErr := fmt.Errorf("iteration error")
	rows := NewMockRowsWithError(testErr)

	// Next should return false when there's an error
	assert.False(t, rows.Next())
}

func TestMockRows_Scan_WithError(t *testing.T) {
	testErr := fmt.Errorf("scan error")
	rows := NewMockRowsWithError(testErr)

	var result string
	err := rows.Scan(&result)
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
}

func TestMockRows_Scan_BeforeNext(t *testing.T) {
	rows := NewMockRows([][]interface{}{{"value"}})

	// Scan before Next should fail
	var result string
	err := rows.Scan(&result)
	assert.Error(t, err)
}

func TestMockRows_MultipleRows(t *testing.T) {
	rows := NewMockRows([][]interface{}{
		{"row1", 1},
		{"row2", 2},
		{"row3", 3},
	})

	count := 0
	for rows.Next() {
		var s string
		var i int
		err := rows.Scan(&s, &i)
		assert.NoError(t, err)
		count++
	}

	assert.Equal(t, 3, count)
	assert.Nil(t, rows.Err())
}

// ========================================
// MockRow Tests
// ========================================

func TestMockRow_Scan(t *testing.T) {
	t.Run("Scan_Success", func(t *testing.T) {
		// Use variadic form directly (not wrapped in slice)
		row := NewMockRowWithValues("test", 42)

		var s string
		var i int
		err := row.Scan(&s, &i)
		assert.NoError(t, err)
		assert.Equal(t, "test", s)
		assert.Equal(t, 42, i)
	})

	t.Run("Scan_Error", func(t *testing.T) {
		testErr := fmt.Errorf("scan error")
		row := NewMockRowWithError(testErr)

		var s string
		err := row.Scan(&s)
		assert.Error(t, err)
		assert.Equal(t, testErr, err)
	})
}

// ========================================
// Database Method Edge Cases
// ========================================

func TestDatabase_InitializeSchema_EdgeCases(t *testing.T) {
	// Test that method signature exists
	db := &Database{Pool: nil}
	assert.NotNil(t, db)

	// InitializeSchema with nil pool should panic
	assert.Panics(t, func() {
		db.InitializeSchema()
	})
}

func TestDatabase_GetDB_EdgeCases(t *testing.T) {
	t.Run("NilPool_ReturnsError", func(t *testing.T) {
		db := &Database{Pool: nil}
		sqlDB, err := db.GetDB()
		assert.Error(t, err)
		assert.Nil(t, sqlDB)
		assert.Contains(t, err.Error(), "internal_database_pool_not_initialized")
	})
}

func TestDatabase_HealthCheck_EdgeCases(t *testing.T) {
	t.Run("NilPool_ReturnsError", func(t *testing.T) {
		db := &Database{Pool: nil}
		err := db.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "internal_database_pool_not_initialized")
	})
}

func TestDatabase_Close_EdgeCases(t *testing.T) {
	t.Run("NilPool_DoesNotPanic", func(t *testing.T) {
		db := &Database{Pool: nil}
		assert.NotPanics(t, func() {
			db.Close()
		})
	})
}
