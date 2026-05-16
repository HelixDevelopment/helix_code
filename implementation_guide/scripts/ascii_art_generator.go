package main

import (
	"fmt"
	"os"
)

func main() {
	logo := GenerateHelixCodeLogo()
	fmt.Println(logo)
}

func GenerateHelixCodeLogo() string {
	return `
[32m    ██╗  ██╗███████╗██╗     ██╗██╗  ██╗     ██████╗ ██████╗ ██████╗ ███████╗[0m
[32m    ██║  ██║██╔════╝██║     ██║╚██╗██╔╝    ██╔════╝██╔═══██╗██╔══██╗██╔════╝[0m
[32m    ███████║█████╗  ██║     ██║ ╚███╔╝     ██║     ██║   ██║██║  ██║█████╗  [0m
[32m    ██╔══██║██╔══╝  ██║     ██║ ██╔██╗     ██║     ██║   ██║██║  ██║██╔══╝  [0m
[32m    ██║  ██║███████╗███████╗██║██╔╝ ██╗    ╚██████╗╚██████╔╝██████╔╝███████╗[0m
[32m    ╚═╝  ╚═╝╚══════╝╚══════╝╚═╝╚═╝  ╚═╝     ╚═════╝ ╚═════╝ ╚═════╝ ╚══════╝[0m

[36m          Distributed AI Development Platform - v1.0.0[0m
[33m    ====================================================[0m
[37m    • Multi-Provider LLM Integration[0m
[37m    • Distributed Worker Network[0m
[37m    • Advanced Tool Calling & Reasoning[0m
[37m    • Cross-Platform Client Support[0m
[37m    • Real-time Collaboration[0m
[33m    ====================================================[0m
`
}

func GenerateProgressBar(progress float64, width int) string {
	filled := int(progress * float64(width))
	bar := ""
	
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "[42m [0m" // Green background
		} else {
			bar += "[47m [0m" // White background
		}
	}
	
	return fmt.Sprintf("[%s] %.1f%%", bar, progress*100)
}

func GenerateWorkerStatus(workers []WorkerStatus) string {
	status := "\n[36mWorker Status:[0m\n"
	status += "[33m╔══════════════════════════════════════════════════════════╗\u001b[0m\n"
	
	for _, worker := range workers {
		var statusColor, statusText string
		switch worker.Status {
		case "active":
			statusColor = "[32m" // Green
			statusText = "● ACTIVE"
		case "offline":
			statusColor = "[31m" // Red
			statusText = "● OFFLINE"
		case "degraded":
			statusColor = "[33m" // Yellow
			statusText = "● DEGRADED"
		default:
			statusColor = "[37m" // White
			statusText = "● UNKNOWN"
		}
		
		status += fmt.Sprintf("[33m║[0m %s%-12s[0m %-20s %-8s %-6s [33m║\u001b[0m\n",
			statusColor, statusText, worker.Hostname, 
			fmt.Sprintf("CPU:%.1f%%", worker.CPUUsage),
			fmt.Sprintf("Tasks:%d", worker.CurrentTasks))
	}
	
	status += "[33m╚══════════════════════════════════════════════════════════╝\u001b[0m"
	return status
}

type WorkerStatus struct {
	Hostname     string
	Status       string
	CPUUsage     float64
	CurrentTasks int
}

func GenerateTaskStatus(tasks []TaskStatus) string {
	status := "\n[36mActive Tasks:[0m\n"
	status += "[33m╔══════════════════════════════════════════════════════════════════════════════╗\u001b[0m\n"
	
	for _, task := range tasks {
		var statusColor, statusText string
		switch task.Status {
		case "running":
			statusColor = "[32m" // Green
			statusText = "▶ RUNNING"
		case "paused":
			statusColor = "[33m" // Yellow
			statusText = "⏸ PAUSED"
		case "waiting":
			statusColor = "[36m" // Cyan
			statusText = "⏳ WAITING"
		case "failed":
			statusColor = "[31m" // Red
			statusText = "✗ FAILED"
		default:
			statusColor = "[37m" // White
			statusText = "? UNKNOWN"
		}
		
		progressBar := GenerateProgressBar(task.Progress, 20)
		status += fmt.Sprintf("[33m║[0m %s%-10s[0m %-25s %-30s [33m║\u001b[0m\n",
			statusColor, statusText, task.Name, progressBar)
	}
	
	status += "[33m╚══════════════════════════════════════════════════════════════════════════════╝\u001b[0m"
	return status
}

type TaskStatus struct {
	Name     string
	Status   string
	Progress float64
}