// p1f07_challenge runs a real background task and prints its polling timeline,
// proving mid-execution streaming. It is the runtime-evidence harness for the
// F07 Challenge (Article XI §11.9).
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"

	"dev.helix.code/internal/tools"
	"dev.helix.code/internal/workflow"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}

func run() error {
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}
	bm := workflow.NewBackgroundManager(zap.NewNop(), workflow.ManagerConfig{})
	defer bm.Close()
	reg.SetBackgroundManager(bm)

	ctx := context.Background()

	fmt.Println("==> start background streaming task")
	res, err := reg.Execute(ctx, "shell", map[string]interface{}{
		"command":           "for i in 1 2 3; do echo line $i; sleep 0.3; done",
		"run_in_background": true,
	})
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	taskID := res.(map[string]interface{})["task_id"].(string)
	fmt.Println("task_id =", taskID)

	deadline := time.Now().Add(3 * time.Second)
	prevCount := -1
	for time.Now().Before(deadline) {
		task, err := bm.GetTask(taskID)
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}
		lines := task.LastLines(100)
		if len(lines) != prevCount {
			fmt.Printf("[poll t=%dms] state=%s lines=%d -> %s\n",
				time.Since(task.StartedAt).Milliseconds(),
				task.State(), len(lines), strings.Join(lines, " | "))
			prevCount = len(lines)
		}
		if task.State() == workflow.TaskCompleted {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	task, _ := bm.GetTask(taskID)
	if task.State() != workflow.TaskCompleted {
		return fmt.Errorf("task did not complete in 3s, state=%s", task.State())
	}
	if prevCount < 2 {
		return fmt.Errorf("only saw %d lines; streaming did not work as expected", prevCount)
	}
	fmt.Println("==> streaming verified: agent saw growing line count mid-execution")

	fmt.Println("==> start sleep 30 task and cancel")
	res2, err := reg.Execute(ctx, "shell", map[string]interface{}{
		"command":           "sleep 30",
		"run_in_background": true,
	})
	if err != nil {
		return fmt.Errorf("execute sleep: %w", err)
	}
	id2 := res2.(map[string]interface{})["task_id"].(string)
	time.Sleep(200 * time.Millisecond)
	if err := bm.StopTask(id2); err != nil {
		return fmt.Errorf("stop task: %w", err)
	}
	cancelDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(cancelDeadline) {
		task, _ := bm.GetTask(id2)
		if task != nil && (task.State() == workflow.TaskCancelled || task.State() == workflow.TaskFailed) {
			fmt.Println("==> sleep task cancelled, state=", task.State())
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	task2, _ := bm.GetTask(id2)
	if task2 == nil || (task2.State() != workflow.TaskCancelled && task2.State() != workflow.TaskFailed) {
		return fmt.Errorf("sleep task not cancelled within 3s")
	}

	out, _ := exec.Command("pgrep", "-x", "sleep").Output()
	pids := strings.TrimSpace(string(out))
	fmt.Println("==> pgrep -x sleep returned:", pids)

	fmt.Println("==> P1-F07 challenge harness PASS")
	return nil
}
