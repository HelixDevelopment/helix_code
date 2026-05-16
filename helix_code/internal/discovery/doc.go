// Copyright 2024 HelixCode. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

/*
Package discovery provides service registration, discovery, and health monitoring
for the HelixCode platform.

# Overview

The discovery package implements a complete service discovery system with multiple
discovery strategies, a service registry with TTL-based expiration, UDP multicast
broadcast discovery, port allocation, and protocol-specific health checking.

# Key Components

The package consists of several interconnected components:

  - ServiceRegistry: Central registry for service registration and lookup
  - DiscoveryClient: Client for discovering services using multiple strategies
  - BroadcastService: UDP multicast service for LAN-based discovery
  - PortAllocator: Dynamic port allocation and management
  - HealthMonitor: Protocol-specific health checking

# Service Registry

ServiceRegistry manages service registration with TTL-based expiration:

	registry := discovery.NewServiceRegistry(discovery.DefaultRegistryConfig())
	registry.Start()
	defer registry.Stop()

	// Register a service
	err := registry.Register(discovery.ServiceInfo{
	    Name:     "api-server",
	    Host:     "localhost",
	    Port:     8080,
	    Protocol: "http",
	    TTL:      30 * time.Second,
	})

	// Query services
	service, err := registry.Get("api-server")
	healthyServices := registry.ListHealthy()

# Discovery Client

DiscoveryClient provides multi-strategy service discovery:

	client := discovery.NewDiscoveryClient(discovery.DefaultDiscoveryClientConfig(
	    registry,
	    portAllocator,
	))

	// Discover a service
	result, err := client.Discover("database")

	// Wait for a service to become available
	result, err := client.WaitForService("cache", 30*time.Second)

	// Get service address directly
	address, err := client.GetServiceAddress("api")

# Discovery Strategies

The package supports multiple discovery strategies executed in order:

  - StrategyDefaultPort: Checks well-known default ports
  - StrategyRegistry: Queries the service registry
  - StrategyBroadcast: Uses UDP multicast discovery
  - StrategyDNS: Falls back to DNS resolution

Default ports are automatically assigned based on service type:

  - database/postgres: 5432
  - cache/redis: 6379
  - api/http: 8080
  - grpc: 9090
  - metrics/prometheus: 9100

# Broadcast Discovery

BroadcastService enables zero-configuration discovery on local networks:

	broadcast := discovery.NewBroadcastService(discovery.DefaultBroadcastConfig())
	broadcast.SetLocalService(serviceInfo)
	broadcast.Start()
	defer broadcast.Stop()

	// Discover services via broadcast
	discovered := broadcast.List()

Messages are exchanged via UDP multicast at 239.255.0.1:7001 by default.

# Health Checking

The registry performs protocol-specific health checks:

  - HTTP/HTTPS: GET request to /health endpoint
  - gRPC: Standard gRPC health checking protocol
  - TCP/UDP: Connection establishment check

Health check configuration:

	config := discovery.RegistryConfig{
	    DefaultTTL:          30 * time.Second,
	    CleanupInterval:     10 * time.Second,
	    EnableHealthChecks:  true,
	    HealthCheckInterval: 15 * time.Second,
	}

# Port Allocation

PortAllocator manages dynamic port allocation:

	allocator := discovery.NewPortAllocator(discovery.DefaultPortAllocatorConfig())

	// Allocate a port (tries preferred, then finds available)
	port, err := allocator.AllocatePort("my-service", 8080)

	// Release when done
	allocator.ReleaseServicePort("my-service")

# Service Information

ServiceInfo contains comprehensive service metadata:

	type ServiceInfo struct {
	    Name          string            // Service name
	    Host          string            // Host address
	    Port          int               // Port number
	    Protocol      string            // tcp, udp, http, https, grpc
	    Version       string            // Service version
	    Metadata      map[string]string // Custom metadata
	    RegisteredAt  time.Time         // Registration time
	    LastHeartbeat time.Time         // Last heartbeat
	    TTL           time.Duration     // Time to live
	    Healthy       bool              // Health status
	}

# Configuration

Discovery settings are configured via config.yaml:

	discovery:
	  enabled: true
	  auto_scan: true
	  scan_interval: 5m
	  scanners:
	    - database
	    - redis
	    - workers

# Thread Safety

All components are thread-safe and designed for concurrent access.
The ServiceRegistry and BroadcastService run background goroutines
for cleanup, health checking, and announcement.

# Error Handling

The package defines specific error types:

  - ErrServiceNotFound: Service not in registry
  - ErrServiceAlreadyRegistered: Duplicate registration
  - ErrInvalidServiceInfo: Invalid service information
  - ErrServiceUnavailable: Service cannot be discovered
  - ErrBroadcastNotRunning: Broadcast service not started
*/
package discovery
