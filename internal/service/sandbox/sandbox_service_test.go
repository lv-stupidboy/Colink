package sandbox

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/anthropic/isdp/internal/model"
	"github.com/google/uuid"
)

func TestSandboxServiceRunStateHelpers(t *testing.T) {
	service := NewSandboxService(nil, nil)
	threadA := uuid.New()
	threadB := uuid.New()
	runA := &SandboxRun{ID: uuid.New(), ThreadID: threadA, Status: model.SandboxStatusRunning}
	runB := &SandboxRun{ID: uuid.New(), ThreadID: threadB, Status: model.SandboxStatusRunning}

	service.activeRuns[runA.ID] = runA
	service.activeRuns[runB.ID] = runB

	got, err := service.GetRunStatus(context.Background(), runA.ID)
	if err != nil {
		t.Fatalf("GetRunStatus returned error: %v", err)
	}
	if got != runA {
		t.Fatalf("GetRunStatus returned wrong run")
	}

	if _, err := service.GetRunStatus(context.Background(), uuid.New()); !errors.Is(err, ErrRunNotFound) {
		t.Fatalf("GetRunStatus missing run error = %v", err)
	}

	if runs := service.ListActiveRuns(threadA); len(runs) != 1 || runs[0].ID != runA.ID {
		t.Fatalf("ListActiveRuns(threadA) = %#v", runs)
	}
	if runs := service.ListActiveRuns(uuid.Nil); len(runs) != 2 {
		t.Fatalf("ListActiveRuns(all) length = %d", len(runs))
	}
}

func TestSandboxServiceProjectServerHelpers(t *testing.T) {
	service := NewSandboxService(nil, nil)
	threadID := uuid.New()
	serverID := uuid.New()
	cancelCalled := false
	killCalled := false
	service.activeServers[serverID] = &ProjectServer{
		ID:         serverID,
		ThreadID:   threadID,
		Mode:       RunModeLocal,
		Status:     "running",
		CancelFunc: func() { cancelCalled = true },
		KillFunc:   func() error { killCalled = true; return nil },
	}

	server, err := service.GetProjectServer(context.Background(), serverID)
	if err != nil {
		t.Fatalf("GetProjectServer returned error: %v", err)
	}
	if server.ID != serverID {
		t.Fatalf("GetProjectServer ID = %s", server.ID)
	}

	byThread, err := service.GetProjectServerByThread(context.Background(), threadID)
	if err != nil {
		t.Fatalf("GetProjectServerByThread returned error: %v", err)
	}
	if byThread.ID != serverID {
		t.Fatalf("GetProjectServerByThread ID = %s", byThread.ID)
	}

	if servers := service.ListProjectServers(); len(servers) != 1 || servers[0].ID != serverID {
		t.Fatalf("ListProjectServers = %#v", servers)
	}

	if logs, err := service.GetServerLogs(context.Background(), serverID); err != nil || logs != "" {
		t.Fatalf("GetServerLogs(local) = %q, %v", logs, err)
	}

	if err := service.StopProject(context.Background(), serverID); err != nil {
		t.Fatalf("StopProject returned error: %v", err)
	}
	if !killCalled || !cancelCalled {
		t.Fatalf("StopProject kill=%v cancel=%v", killCalled, cancelCalled)
	}
	if _, ok := service.activeServers[serverID]; ok {
		t.Fatalf("StopProject did not remove server")
	}
	if err := service.StopProject(context.Background(), serverID); !errors.Is(err, ErrServerNotFound) {
		t.Fatalf("StopProject missing server error = %v", err)
	}
}

func TestSandboxServiceRunProjectValidationAndHelpers(t *testing.T) {
	service := NewSandboxService(nil, nil)

	if _, err := service.RunProject(context.Background(), &RunProjectRequest{}); err == nil || !strings.Contains(err.Error(), "project path is required") {
		t.Fatalf("RunProject empty path error = %v", err)
	}
	if _, err := service.RunProject(context.Background(), &RunProjectRequest{ProjectPath: t.TempDir(), Mode: RunMode("bad")}); err == nil || !strings.Contains(err.Error(), "unsupported run mode") {
		t.Fatalf("RunProject bad mode error = %v", err)
	}

	cases := map[string]string{
		"node":    "node:20-alpine",
		"react":   "node:22-alpine",
		"vue":     "node:22-alpine",
		"python":  "python:3.11-slim",
		"go":      "golang:1.21-alpine",
		"static":  "python:3.11-slim",
		"unknown": "node:20-alpine",
	}
	for projectType, want := range cases {
		if got := service.getDockerImage(projectType); got != want {
			t.Fatalf("getDockerImage(%q) = %q, want %q", projectType, got, want)
		}
	}

	startCases := map[string]struct {
		cmd  string
		args []string
	}{
		"react":  {"sh", []string{"-c", "cd /workspace && npm run dev -- --host 0.0.0.0"}},
		"vue":    {"sh", []string{"-c", "cd /workspace && npm run dev -- --host 0.0.0.0"}},
		"node":   {"sh", []string{"-c", "cd /workspace && npm start"}},
		"python": {"sh", []string{"-c", "cd /workspace && python -m http.server 8080"}},
		"go":     {"sh", []string{"-c", "cd /workspace && go run ."}},
		"static": {"sh", []string{"-c", "cd /workspace && python -m http.server 8080"}},
	}
	for projectType, want := range startCases {
		cmd, args := service.getDockerStartCommand(projectType)
		if cmd != want.cmd || strings.Join(args, "\x00") != strings.Join(want.args, "\x00") {
			t.Fatalf("getDockerStartCommand(%q) = %q %#v", projectType, cmd, args)
		}
	}
	if cmd, args := service.getDockerStartCommand("unknown"); cmd != "" || args != nil {
		t.Fatalf("getDockerStartCommand(unknown) = %q %#v", cmd, args)
	}

	port, err := service.getAvailablePort()
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("local port binding is not permitted in this sandbox: %v", err)
		}
		t.Fatalf("getAvailablePort returned error: %v", err)
	}
	if port <= 0 {
		t.Fatalf("getAvailablePort returned invalid port %d", port)
	}
	if err := service.waitForServer(port, 10*time.Millisecond); err == nil {
		t.Fatalf("waitForServer on closed port unexpectedly succeeded")
	}
	if err := service.killProcessOnPort(port); err != nil {
		t.Fatalf("killProcessOnPort on free port returned error: %v", err)
	}
	if got := intPtr(7); got == nil || *got != 7 {
		t.Fatalf("intPtr = %v", got)
	}
	if service.IsDockerAvailable() {
		t.Fatalf("nil docker client should not be available")
	}
}

func TestSandboxServiceWaitForServerSuccess(t *testing.T) {
	service := NewSandboxService(nil, nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("local port binding is not permitted in this sandbox: %v", err)
		}
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	if err := service.waitForServer(port, 600*time.Millisecond); err != nil {
		t.Fatalf("waitForServer returned error: %v", err)
	}
}

func TestLocalProcessRunnerDetectProjectTypeAndStartCommand(t *testing.T) {
	root := t.TempDir()
	runner := NewLocalProcessRunner(root)

	write := func(rel, content string) string {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
		return filepath.Dir(path)
	}

	reactDir := write("react/package.json", `{"dependencies":{"react":"latest"}}`)
	vueDir := write("vue/package.json", `{"dependencies":{"vue":"latest"}}`)
	nodeDir := write("node/package.json", `{"scripts":{"start":"node index.js"}}`)
	pythonDir := write("python/requirements.txt", "flask\n")
	write("python/app.py", "from flask import Flask\napp = Flask(__name__)\n")
	fastAPIDir := write("fastapi/requirements.txt", "fastapi\n")
	write("fastapi/main.py", "from fastapi import FastAPI\napp = FastAPI()\n")
	goDir := write("go/go.mod", "module example.com/app\n")
	staticDir := write("static/index.html", "<h1>ok</h1>")

	tests := []struct {
		path     string
		wantType string
		wantCmd  string
		wantArgs []string
		wantPort int
	}{
		{reactDir, "react", "npm", []string{"run", "dev"}, 5173},
		{vueDir, "vue", "npm", []string{"run", "dev"}, 5173},
		{nodeDir, "node", "npm", []string{"start"}, 3000},
		{pythonDir, "python", "python", []string{"app.py"}, 5000},
		{fastAPIDir, "python", "uvicorn", []string{"main:app", "--reload"}, 8000},
		{goDir, "go", "go", []string{"run", "."}, 3001},
		{staticDir, "static", "python", []string{"-m", "http.server", "3002"}, 3002},
	}

	for _, tt := range tests {
		t.Run(tt.wantType+"_"+filepath.Base(tt.path), func(t *testing.T) {
			if got := runner.DetectProjectType(tt.path); got != tt.wantType {
				t.Fatalf("DetectProjectType = %q, want %q", got, tt.wantType)
			}
			cmd, args, port := runner.GetStartCommand(tt.path)
			if cmd != tt.wantCmd || strings.Join(args, "\x00") != strings.Join(tt.wantArgs, "\x00") || port != tt.wantPort {
				t.Fatalf("GetStartCommand = %q %#v %d", cmd, args, port)
			}
		})
	}

	if got := runner.DetectProjectType(filepath.Join(root, "missing")); got != "unknown" {
		t.Fatalf("DetectProjectType missing = %q", got)
	}
	if cmd, args, port := runner.GetStartCommand(filepath.Join(root, "missing")); cmd != "" || args != nil || port != 0 {
		t.Fatalf("GetStartCommand unknown = %q %#v %d", cmd, args, port)
	}
}

func TestLocalProcessRunnerRelativePathsAndCommands(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runner := NewLocalProcessRunner(root)
	if runner.workDir != root {
		t.Fatalf("workDir = %q", runner.workDir)
	}
	if got := NewLocalProcessRunner("").workDir; got != "./repos" {
		t.Fatalf("default workDir = %q", got)
	}

	cmd, args := shellCommand("pwd")
	result, err := runner.Run(context.Background(), "project", cmd, args...)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if strings.TrimSpace(result.Output) != project {
		t.Fatalf("Run output = %q, want %q", result.Output, project)
	}
	if result.ExitCode != 0 || result.ID == "" || result.EndedAt.Before(result.StartedAt) {
		t.Fatalf("Run result = %#v", result)
	}

	failCmd, failArgs := shellCommand("exit 3")
	failed, err := runner.Run(context.Background(), "project", failCmd, failArgs...)
	if err != nil {
		t.Fatalf("Run failing command returned setup error: %v", err)
	}
	if failed.ExitCode != 3 || failed.Error == "" {
		t.Fatalf("Run failing result = %#v", failed)
	}

	if _, err := runner.Run(context.Background(), "missing", cmd, args...); err == nil || !strings.Contains(err.Error(), "project path not found") {
		t.Fatalf("Run missing project error = %v", err)
	}
}

func TestLocalProcessRunnerRunWithOutputAndKillNoop(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "project"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runner := NewLocalProcessRunner(root)

	cmd, args := shellCommand("echo out; echo err 1>&2")
	var outputs []string
	process, err := runner.RunWithOutput(context.Background(), "project", cmd, func(output string) {
		outputs = append(outputs, output)
	}, args...)
	if err != nil {
		t.Fatalf("RunWithOutput returned error: %v", err)
	}

	select {
	case result := <-process.Result:
		joined := result.Output + result.Error + strings.Join(outputs, "")
		if !strings.Contains(joined, "out") || !strings.Contains(joined, "err") {
			t.Fatalf("RunWithOutput joined output = %q", joined)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunWithOutput timed out")
	}

	if err := (&LocalProcessCmd{}).Kill(); err != nil {
		t.Fatalf("Kill nil process returned error: %v", err)
	}
	if _, err := runner.RunWithOutput(context.Background(), "missing", cmd, nil, args...); err == nil || !strings.Contains(err.Error(), "project path not found") {
		t.Fatalf("RunWithOutput missing project error = %v", err)
	}
}

func TestLocalProcessRunnerInstallDependenciesUnsupported(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "project"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	runner := NewLocalProcessRunner(root)

	if _, err := runner.InstallDependencies(context.Background(), "project", "ruby"); err == nil || !strings.Contains(err.Error(), "unsupported project type") {
		t.Fatalf("InstallDependencies unsupported error = %v", err)
	}
}

func TestListProjectServersOrderAgnostic(t *testing.T) {
	service := NewSandboxService(nil, nil)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, id := range ids {
		service.activeServers[id] = &ProjectServer{ID: id}
	}

	got := service.ListProjectServers()
	gotIDs := make([]string, 0, len(got))
	for _, server := range got {
		gotIDs = append(gotIDs, server.ID.String())
	}
	wantIDs := []string{ids[0].String(), ids[1].String(), ids[2].String()}
	sort.Strings(gotIDs)
	sort.Strings(wantIDs)
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("ListProjectServers IDs = %v, want %v", gotIDs, wantIDs)
	}
}

func shellCommand(script string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", script}
	}
	return "sh", []string{"-c", script}
}
