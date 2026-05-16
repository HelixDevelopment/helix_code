// Copyright 2024 HelixCode. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

/*
Package deployment provides comprehensive production deployment orchestration
for the HelixCode platform.

# Overview

The deployment package handles multi-target deployment automation with support for
various deployment strategies, security gates, performance validation, health checking,
and automatic rollback capabilities. It provides a complete deployment pipeline that
ensures safe and reliable production deployments.

# Key Types

ProductionDeployer is the main orchestrator that manages the entire deployment lifecycle:

	deployer, err := deployment.NewProductionDeployer(config)
	if err != nil {
	    log.Fatal(err)
	}
	status, err := deployer.StartProductionDeployment(ctx)

DeploymentConfig defines the deployment configuration including strategy, target servers,
security and performance gates, and notification settings.

DeploymentStatus tracks the comprehensive state of a deployment including phases completed,
servers deployed, gate statuses, and metrics.

# Deployment Strategies

The package supports multiple deployment strategies:

  - BlueGreenDeploy: Blue-green deployment with instant switchover
  - CanaryDeploy: Canary deployment with gradual traffic shift
  - RollingDeploy: Rolling deployment updating servers incrementally
  - RecreateDeploy: Recreate strategy stopping all before deploying
  - ProductionDeploy: Direct production deployment

# Deployment Phases

A production deployment executes through several phases:

  - Preparation: Prerequisites check, environment setup, server validation
  - SecurityCheck: Zero-tolerance security gate validation
  - PerformanceCheck: Performance gate validation against targets
  - Deployment: Actual deployment to target servers
  - HealthCheck: Post-deployment health verification
  - Validation: Final deployment validation
  - Monitoring: Production monitoring setup

# Security Gates

When enabled, security gates enforce zero-tolerance security policies:

	config := &DeploymentConfig{
	    SecurityGateEnabled: true,
	    // ... other config
	}

The security gate scans for critical and high-severity issues and blocks deployment
if any critical issues are detected.

# Performance Gates

Performance gates validate that the deployment meets specified targets:

	config := &DeploymentConfig{
	    PerformanceGateEnabled: true,
	    PerformanceGateStatus: PerformanceGateStatus{
	        ThroughputTarget: 2000,     // ops/sec
	        LatencyTarget:    "50ms",
	        CPUTarget:        80.0,     // percentage
	        MemoryTarget:     4 * 1024 * 1024 * 1024, // bytes
	    },
	}

# Health Checks

The package performs health checks on deployed servers to ensure successful deployment.
A deployment requires at least 90% of servers to be healthy by default.

# Automatic Rollback

When enabled, automatic rollback is triggered upon deployment failure:

	config := &DeploymentConfig{
	    AutoRollbackEnabled: true,
	    RollbackTimeout:     5 * time.Minute,
	}

# Notifications

The package supports multi-channel notifications for deployment events:

	config := &DeploymentConfig{
	    Notifications: NotificationConfig{
	        SlackEnabled:    true,
	        SlackWebhookURL: "https://hooks.slack.com/...",
	        EmailEnabled:    true,
	        EmailRecipients: []string{"team@example.com"},
	    },
	}

# Supported Targets

The deployment system supports various deployment targets:

  - Docker/Kubernetes clusters
  - AWS (EC2, ECS, Lambda)
  - Google Cloud (GCE, Cloud Run)
  - Azure (VMs, Container Apps)
  - SSH-accessible servers

# Configuration

Deployment settings are typically configured via config.yaml:

	deployment:
	  default_target: "production"
	  targets:
	    production:
	      type: "kubernetes"
	      config:
	        namespace: "production"
	        replicas: 3
	    staging:
	      type: "docker"
	      config:
	        host: "staging.example.com"

# Thread Safety

The ProductionDeployer uses atomic operations and mutex protection to ensure
safe concurrent access. Only one deployment can run at a time.
*/
package deployment
