// Package logging provides structured logging functionality for the HelixCode platform.
//
// The logging package offers a simple, level-based logging system with support for
// formatted messages, named loggers, and both instance-based and global logging functions.
// It wraps the standard library's log package with severity level filtering.
//
// # Log Levels
//
// The package supports five log levels in increasing severity:
//
//   - DEBUG: Detailed debugging information for development
//   - INFO: General operational information (default level)
//   - WARN: Warning messages for potentially harmful situations
//   - ERROR: Error messages for serious problems
//   - FATAL: Fatal errors that cause program termination
//
// # Basic Usage
//
// Creating and using a logger:
//
//	// Create with specific level
//	logger := logging.NewLogger(logging.DEBUG)
//
//	// Log at different levels
//	logger.Debug("Processing item %d", itemID)
//	logger.Info("Server started on port %d", port)
//	logger.Warn("Connection pool running low: %d/%d", used, max)
//	logger.Error("Failed to connect: %v", err)
//	logger.Fatal("Critical failure: %v", err) // Exits program
//
// # Named Loggers
//
// Create loggers with component names for easier filtering:
//
//	authLogger := logging.NewLoggerWithName("auth")
//	dbLogger := logging.NewLoggerWithName("database")
//	apiLogger := logging.NewLoggerWithName("api")
//
//	// Output includes the name as prefix:
//	// [auth] 2024/01/15 10:30:00 [INFO] User authenticated
//	authLogger.Info("User authenticated")
//
// # Global Logger
//
// For convenience, package-level functions use a default global logger:
//
//	// Use the global logger directly
//	logging.Debug("Debug message")
//	logging.Info("Info message")
//	logging.Warn("Warning message")
//	logging.Error("Error message")
//	logging.Fatal("Fatal message")
//
// # Default Logger
//
// Get a default logger with INFO level:
//
//	logger := logging.DefaultLogger()
//
// # Test Logger
//
// Create loggers suitable for test output:
//
//	logger := logging.NewTestLogger("component-test")
//
// # Log Output Format
//
// Log output follows this format:
//
//	[prefix] YYYY/MM/DD HH:MM:SS [LEVEL] message
//
// Examples:
//
//	2024/01/15 10:30:00 [INFO] Server started on port 8080
//	[auth] 2024/01/15 10:30:01 [DEBUG] Validating token for user-123
//	[database] 2024/01/15 10:30:02 [ERROR] Connection failed: timeout
//
// # Level Filtering
//
// Messages below the logger's configured level are silently discarded:
//
//	logger := logging.NewLogger(logging.WARN)
//
//	logger.Debug("Won't print") // Filtered out
//	logger.Info("Won't print")  // Filtered out
//	logger.Warn("Will print")   // Printed
//	logger.Error("Will print")  // Printed
//
// # Thread Safety
//
// The Logger type is safe for concurrent use from multiple goroutines.
// All logging operations are serialized through the underlying log.Logger.
package logging
