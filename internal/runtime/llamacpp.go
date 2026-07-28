package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/FlameInTheDark/localize-app/internal/domain"
	"github.com/FlameInTheDark/localize-app/internal/operations"
)

type LlamaManager struct {
	mu         sync.Mutex
	runtimeDir string
	operations *operations.Hub
	process    *managedProcess
	endpoint   string
	modelID    string
	mode       domain.RuntimeMode
	version    string
}

func NewLlamaManager(runtimeDir string, hub *operations.Hub) *LlamaManager {
	return &LlamaManager{runtimeDir: runtimeDir, operations: hub}
}

func (m *LlamaManager) Endpoint(ctx context.Context, settings domain.LlamaCppSettings, assignment domain.ModelAssignment, operationID string) (string, error) {
	if assignment.ID == "" || assignment.Path == "" {
		return "", fmt.Errorf("select and install a llama.cpp model in Settings before translating")
	}
	if _, err := os.Stat(assignment.Path); err != nil {
		return "", fmt.Errorf("configured llama.cpp model is missing: %w", err)
	}
	if assignment.ProjectionPath != "" {
		if _, err := os.Stat(assignment.ProjectionPath); err != nil {
			return "", fmt.Errorf("configured llama.cpp vision projection is missing: %w", err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process != nil && m.process.Running() && m.modelID == assignment.ID && m.version == settings.RuntimeVersion && (settings.RuntimeMode == domain.RuntimeAuto || m.mode == settings.RuntimeMode) {
		return m.endpoint, nil
	}
	m.stopLocked()
	mode, binary, err := m.chooseBinary(settings.RuntimeVersion, settings.RuntimeMode)
	if err != nil {
		return "", err
	}
	port, err := freePort()
	if err != nil {
		return "", err
	}
	args := serverArguments(settings, assignment, port, mode)
	m.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-runtime", Stage: "starting", Message: "Starting " + string(mode) + " llama.cpp runtime"})
	process, err := startManagedProcess(binary, args)
	if err != nil {
		return "", fmt.Errorf("start llama.cpp: %w", err)
	}
	endpoint := "http://127.0.0.1:" + port
	if err := waitReady(ctx, endpoint+"/v1/models", 45*time.Second, process.Done()); err != nil {
		process.Stop()
		if settings.RuntimeMode == domain.RuntimeAuto && mode == domain.RuntimeCUDA {
			m.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-runtime", Stage: "fallback", Message: "CUDA runtime did not start; falling back to CPU"})
			return m.startCPUFallbackLocked(ctx, settings, assignment, operationID)
		}
		return "", m.startupError(err, process)
	}
	m.process, m.endpoint, m.modelID, m.mode, m.version = process, endpoint, assignment.ID, mode, settings.RuntimeVersion
	m.operations.Emit(domain.OperationProgress{OperationID: operationID, Kind: "llamacpp-runtime", Stage: "ready", Message: "llama.cpp is ready"})
	return endpoint, nil
}

func (m *LlamaManager) startCPUFallbackLocked(ctx context.Context, settings domain.LlamaCppSettings, assignment domain.ModelAssignment, operationID string) (string, error) {
	settings.RuntimeMode = domain.RuntimeCPU
	return m.endpointLocked(ctx, settings, assignment, operationID)
}

func (m *LlamaManager) endpointLocked(ctx context.Context, settings domain.LlamaCppSettings, assignment domain.ModelAssignment, operationID string) (string, error) {
	mode, binary, err := m.chooseBinary(settings.RuntimeVersion, settings.RuntimeMode)
	if err != nil {
		return "", err
	}
	port, err := freePort()
	if err != nil {
		return "", err
	}
	args := serverArguments(settings, assignment, port, mode)
	process, err := startManagedProcess(binary, args)
	if err != nil {
		return "", fmt.Errorf("start CPU fallback: %w", err)
	}
	endpoint := "http://127.0.0.1:" + port
	if err := waitReady(ctx, endpoint+"/v1/models", 45*time.Second, process.Done()); err != nil {
		process.Stop()
		return "", m.startupError(fmt.Errorf("CPU fallback did not become ready: %w", err), process)
	}
	m.process, m.endpoint, m.modelID, m.mode, m.version = process, endpoint, assignment.ID, mode, settings.RuntimeVersion
	return endpoint, nil
}

func (m *LlamaManager) chooseBinary(version string, requested domain.RuntimeMode) (domain.RuntimeMode, string, error) {
	if runtime.GOOS != "windows" {
		return "", "", errors.New("the managed llama.cpp runtime is currently Windows-only")
	}
	version = strings.TrimSpace(version)
	if version == "" {
		return "", "", errors.New("install and select a llama.cpp runtime version in Settings before translating")
	}
	mode := requested
	if mode == domain.RuntimeAuto {
		if _, err := exec.LookPath("nvidia-smi"); err == nil {
			if binary := m.binary(version, domain.RuntimeCUDA); fileExists(binary) {
				return domain.RuntimeCUDA, binary, nil
			}
		}
		mode = domain.RuntimeCPU
	}
	binary := m.binary(version, mode)
	if _, err := os.Stat(binary); err != nil {
		return "", "", fmt.Errorf("%s llama.cpp runtime %s is not installed; install it in Settings", mode, version)
	}
	return mode, binary, nil
}

func (m *LlamaManager) binary(version string, mode domain.RuntimeMode) string {
	return filepath.Join(m.runtimeDir, version, string(mode), "llama-server.exe")
}
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func serverArguments(settings domain.LlamaCppSettings, assignment domain.ModelAssignment, port string, mode domain.RuntimeMode) []string {
	args := []string{"-m", assignment.Path, "--host", "127.0.0.1", "--port", port, "--alias", assignment.ID, "-c", fmt.Sprint(settings.ContextSize)}
	if assignment.ProjectionPath != "" {
		args = append(args, "--mmproj", assignment.ProjectionPath)
	}
	if domain.IsTranslateGemmaModel(assignment.ID + " " + assignment.Path) {
		// TranslateGemma's embedded template requires a non-standard message
		// shape. The runtime's automatic template parser probes it using a
		// normal OpenAI message and exits before opening its API. The Gemma
		// compatibility template is a safe, user-message-only fallback; the
		// inference adapter preserves the translation instructions in that user
		// message below.
		args = append(args, "--no-jinja", "--chat-template", "gemma")
	}
	if mode == domain.RuntimeCUDA || mode == domain.RuntimeVulkan || mode == domain.RuntimeHIP {
		args = append(args, "-ngl", "999")
	}
	return args
}

func (m *LlamaManager) Stop() { m.mu.Lock(); defer m.mu.Unlock(); m.stopLocked() }
func (m *LlamaManager) stopLocked() {
	if m.process != nil {
		m.process.Stop()
	}
	m.process, m.endpoint, m.modelID, m.version = nil, "", "", ""
}
func (m *LlamaManager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.process != nil && m.process.Running()
}

func freePort() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() { _ = listener.Close() }()
	return fmt.Sprint(listener.Addr().(*net.TCPAddr).Port), nil
}
func waitReady(ctx context.Context, endpoint string, timeout time.Duration, processDone <-chan struct{}) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err == nil {
			if response, err := client.Do(req); err == nil {
				_ = response.Body.Close()
				if response.StatusCode/100 == 2 {
					return nil
				}
			}
		}
		select {
		case <-processDone:
			return errors.New("the llama.cpp process exited before opening its API")
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("timed out")
		case <-ticker.C:
		}
	}
}

func (m *LlamaManager) startupError(cause error, process *managedProcess) error {
	if detail := process.Output(); detail != "" {
		return fmt.Errorf("llama.cpp did not become ready: %w. Runtime output: %s", cause, detail)
	}
	return fmt.Errorf("llama.cpp did not become ready: %w", cause)
}

// managedProcess owns one child started by Localize and makes an early exit
// observable without racing exec.Cmd.Wait against shutdown.
type managedProcess struct {
	command *exec.Cmd
	done    chan struct{}
	output  *processOutput

	mu  sync.RWMutex
	err error
}

func startManagedProcess(binary string, args []string) (*managedProcess, error) {
	output := &processOutput{}
	command := exec.Command(binary, args...)
	command.Dir = filepath.Dir(binary)
	configureHiddenChildProcess(command)
	command.Stdout = io.MultiWriter(os.Stderr, output)
	command.Stderr = io.MultiWriter(os.Stderr, output)
	if err := command.Start(); err != nil {
		return nil, err
	}
	process := &managedProcess{command: command, done: make(chan struct{}), output: output}
	go func() {
		err := command.Wait()
		process.mu.Lock()
		process.err = err
		process.mu.Unlock()
		close(process.done)
	}()
	return process, nil
}

func (p *managedProcess) Done() <-chan struct{} { return p.done }
func (p *managedProcess) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
func (p *managedProcess) Stop() {
	if p == nil {
		return
	}
	if p.Running() && p.command.Process != nil {
		_ = p.command.Process.Kill()
	}
	<-p.done
}
func (p *managedProcess) Output() string { return p.output.String() }

// processOutput keeps the final diagnostic bounded while accepting concurrent
// stdout and stderr writes from exec.Cmd.
type processOutput struct {
	mu   sync.Mutex
	data []byte
}

func (o *processOutput) Write(data []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.data = append(o.data, data...)
	const maxOutput = 6000
	if len(o.data) > maxOutput {
		o.data = append([]byte(nil), o.data[len(o.data)-maxOutput:]...)
	}
	return len(data), nil
}
func (o *processOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return strings.TrimSpace(bytes.NewBuffer(o.data).String())
}

func (m *LlamaManager) Describe() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.endpoint == "" {
		return "Not running"
	}
	return strings.TrimPrefix(m.endpoint, "http://") + " (" + string(m.mode) + ")"
}
