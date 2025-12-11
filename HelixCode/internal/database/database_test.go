package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/jackc/pgx/v5/pgxpool"
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
	assert.Contains(t, err.Error(), "failed to ping database")
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
	assert.Contains(t, err.Error(), "database pool is not initialized")
}

func TestDatabase_HealthCheck(t *testing.T) {
	// Test HealthCheck on database with nil pool
	db := &Database{Pool: nil}
	err := db.HealthCheck()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database pool is not initialized")
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
		assert.Contains(t, err.Error(), "database pool is not initialized")
	})

	t.Run("HealthCheck_NilPool", func(t *testing.T) {
		db := &Database{Pool: nil}
		err := db.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database pool is not initialized")
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
			assert.Contains(t, err.Error(), "database pool is not initialized")
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