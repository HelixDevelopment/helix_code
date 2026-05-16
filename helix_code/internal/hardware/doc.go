// Copyright 2024 HelixCode. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

/*
Package hardware provides hardware detection and capability profiling
for the HelixCode platform.

# Overview

The hardware package detects system hardware capabilities including CPU, GPU,
memory, and platform information. It is used to determine optimal configurations
for running LLM models and to provide hardware-aware compilation flags.

# Hardware Detection

Detector performs comprehensive hardware detection:

	detector := hardware.NewDetector()
	info, err := detector.Detect()
	if err != nil {
	    log.Printf("Detection error: %v", err)
	}

	log.Printf("CPU: %s (%d cores)", info.CPU.Model, info.CPU.Cores)
	log.Printf("GPU: %s (%s VRAM)", info.GPU.Model, info.GPU.VRAM)
	log.Printf("Platform: %s/%s", info.Platform.OS, info.Platform.Architecture)

# Hardware Profile

HardwareProfile provides a simplified view of system capabilities:

	detector := hardware.NewHardwareDetector()
	profile := detector.GetProfile()

	// Or use default profile
	profile := hardware.DefaultProfile()

HardwareProfile contains:
  - CPU information (cores, threads, architecture, features)
  - GPU information (if available)
  - Memory information (total, available, swap)
  - OS information (name, version, architecture)
  - Network information (internet connectivity, latency)

# CPU Information

CPUInfo contains detailed CPU specifications:

	type CPUInfo struct {
	    Architecture string // CPU architecture
	    Vendor       string // CPU vendor (Intel, AMD, Apple, etc.)
	    Model        string // CPU model name
	    Cores        int    // Physical cores
	    Threads      int    // Logical threads
	    HasAVX       bool   // AVX support
	    HasAVX2      bool   // AVX2 support
	    HasNEON      bool   // ARM NEON support
	    Arch         string // Go architecture (amd64, arm64, etc.)
	    Frequency    int    // Clock frequency in MHz
	    CacheSize    int    // Cache size in KB
	}

# GPU Information

GPUInfo contains GPU specifications:

	type GPUInfo struct {
	    Name              string  // Full GPU name
	    Type              GPUType // nvidia, amd, apple, intel
	    Vendor            string  // GPU vendor
	    Model             string  // GPU model
	    VRAM              string  // Video RAM (e.g., "8GB", "16GB")
	    ComputeCapability float64 // CUDA compute capability
	    CUDAVersion       string  // CUDA version if NVIDIA
	    SupportsCUDA      bool    // CUDA support
	    SupportsMetal     bool    // Metal support (macOS)
	}

# GPU Types

The package supports various GPU types:

  - GPUTypeNVIDIA: NVIDIA GPUs with CUDA support
  - GPUTypeAMD: AMD GPUs
  - GPUTypeApple: Apple Silicon GPUs with Metal support
  - GPUTypeIntel: Intel integrated GPUs

# Optimal Model Size

Determine the optimal LLM model size for the hardware:

	detector := hardware.NewDetector()
	detector.Detect()

	modelSize := detector.GetOptimalModelSize()
	// Returns: "3B", "7B", "13B", "34B", or "70B"

Model size recommendations based on total available memory:
  - 32GB+: 70B parameter models
  - 16GB+: 34B parameter models
  - 8GB+: 13B parameter models
  - 4GB+: 7B parameter models
  - <4GB: 3B parameter models

# Model Compatibility

Check if hardware can run a specific model:

	canRun := detector.CanRunModel("13B")
	if !canRun {
	    log.Println("Hardware insufficient for 13B model")
	}

# Compilation Flags

Get hardware-specific compilation flags for llama.cpp:

	flags := detector.GetCompilationFlags()
	// May return: ["-DGGML_USE_CUBLAS"] for NVIDIA
	// Or: ["-DGGML_USE_METAL"] for Apple Silicon

# Platform-Specific Detection

The package uses platform-specific detection methods:

Linux:
  - CPU info from /proc/cpuinfo
  - NVIDIA GPU via nvidia-smi
  - Memory from /proc/meminfo

macOS:
  - CPU info via sysctl
  - GPU info via system_profiler
  - Metal support detection

Windows:
  - Generic detection with runtime package

# Architecture Constants

The package defines architecture constants:

	const (
	    ArchX86_64 Arch = "x86_64"
	    ArchARM64  Arch = "arm64"
	    ArchARM32  Arch = "arm32"
	)

# OS Type Constants

The package defines OS type constants:

	const (
	    OSTypeLinux   OSType = "linux"
	    OSTypeMacOS   OSType = "macos"
	    OSTypeWindows OSType = "windows"
	)

# Configuration

Hardware settings are configured via config.yaml:

	hardware:
	  enabled: true
	  auto_detect: true
	  serial:
	    default_baud: 9600
	  gpio:
	    platform: "raspberry_pi"

# Usage Example

Complete usage example:

	package main

	import (
	    "log"
	    "dev.helix.code/internal/hardware"
	)

	func main() {
	    detector := hardware.NewDetector()
	    info, err := detector.Detect()
	    if err != nil {
	        log.Fatal(err)
	    }

	    log.Printf("System: %s on %s/%s",
	        info.Platform.Hostname,
	        info.Platform.OS,
	        info.Platform.Architecture)

	    log.Printf("CPU: %s (%s) - %d cores",
	        info.CPU.Model,
	        info.CPU.Vendor,
	        info.CPU.Cores)

	    if info.GPU.SupportsCUDA {
	        log.Printf("GPU: %s with %s VRAM (CUDA)", info.GPU.Model, info.GPU.VRAM)
	    } else if info.GPU.SupportsMetal {
	        log.Printf("GPU: %s (Metal)", info.GPU.Model)
	    }

	    log.Printf("Optimal model size: %s", detector.GetOptimalModelSize())
	    log.Printf("Compilation flags: %v", detector.GetCompilationFlags())
	}

# Error Handling

Detection methods log warnings but do not fail the overall detection.
Individual detection failures result in default or unknown values:

	// CPU detection may fail but info is still usable
	info, _ := detector.Detect()
	if info.CPU.Model == "Unknown" {
	    log.Println("CPU model detection failed")
	}

# Thread Safety

The Detector is not thread-safe. Create separate instances for concurrent use
or synchronize access externally.
*/
package hardware
