package simrunner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// IPCClient mirrors backend SimulationIPCClient (filesystem commands/responses).
type IPCClient struct {
	simDir string
}

func NewIPCClient(simDir string) *IPCClient {
	return &IPCClient{simDir: simDir}
}

func (c *IPCClient) commandsDir() string  { return filepath.Join(c.simDir, "ipc_commands") }
func (c *IPCClient) responsesDir() string { return filepath.Join(c.simDir, "ipc_responses") }

func (c *IPCClient) ensureDirs() error {
	if err := os.MkdirAll(c.commandsDir(), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(c.responsesDir(), 0o755)
}

// CheckEnvAlive reads env_status.json status == "alive".
func (c *IPCClient) CheckEnvAlive() bool {
	raw, err := os.ReadFile(filepath.Join(c.simDir, "env_status.json"))
	if err != nil {
		return false
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	return fmt.Sprint(m["status"]) == "alive"
}

func (c *IPCClient) EnvDetail() map[string]any {
	def := map[string]any{
		"status": "stopped", "twitter_available": false, "reddit_available": false, "timestamp": nil,
	}
	raw, err := os.ReadFile(filepath.Join(c.simDir, "env_status.json"))
	if err != nil {
		return def
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return def
	}
	return map[string]any{
		"status":            m["status"],
		"twitter_available": m["twitter_available"],
		"reddit_available":  m["reddit_available"],
		"timestamp":         m["timestamp"],
	}
}

func (c *IPCClient) SendCommand(commandType string, args map[string]any, timeout time.Duration) (map[string]any, error) {
	if err := c.ensureDirs(); err != nil {
		return nil, err
	}
	id := uuid.NewString()
	cmd := map[string]any{
		"command_id":   id,
		"command_type": commandType,
		"args":         args,
		"timestamp":    time.Now().UTC().Format(time.RFC3339Nano),
	}
	cmdPath := filepath.Join(c.commandsDir(), id+".json")
	respPath := filepath.Join(c.responsesDir(), id+".json")
	raw, err := json.MarshalIndent(cmd, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(cmdPath, raw, 0o644); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(respPath); err == nil {
			var resp map[string]any
			if json.Unmarshal(b, &resp) == nil {
				_ = os.Remove(cmdPath)
				_ = os.Remove(respPath)
				return resp, nil
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	_ = os.Remove(cmdPath)
	return nil, fmt.Errorf("timeout waiting for IPC response")
}

func (c *IPCClient) SendBatchInterview(interviews []map[string]any, platform *string, timeout time.Duration) (map[string]any, error) {
	args := map[string]any{"interviews": interviews}
	if platform != nil {
		args["platform"] = *platform
	}
	resp, err := c.SendCommand("batch_interview", args, timeout)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *IPCClient) SendCloseEnv(timeout time.Duration) (map[string]any, error) {
	return c.SendCommand("close_env", map[string]any{}, timeout)
}
