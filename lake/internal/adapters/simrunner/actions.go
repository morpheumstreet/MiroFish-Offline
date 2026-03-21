package simrunner

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Action is one row from actions.jsonl (agent action or ignored event).
type Action struct {
	RoundNum   int            `json:"round"`
	Timestamp  string         `json:"timestamp"`
	Platform   string         `json:"platform"`
	AgentID    int            `json:"agent_id"`
	AgentName  string         `json:"agent_name"`
	ActionType string         `json:"action_type"`
	ActionArgs map[string]any `json:"action_args"`
	Result     any            `json:"result"`
	Success    bool           `json:"success"`
}

func (a Action) ToMap() map[string]any {
	return map[string]any{
		"round_num":   a.RoundNum,
		"timestamp":   a.Timestamp,
		"platform":    a.Platform,
		"agent_id":    a.AgentID,
		"agent_name":  a.AgentName,
		"action_type": a.ActionType,
		"action_args": a.ActionArgs,
		"result":      a.Result,
		"success":     a.Success,
	}
}

// ReadActionsFromJSONL reads new lines from path starting at byte offset; returns new offset.
func ReadActionsFromJSONL(path string, startOffset int64, defaultPlatform string, state *runStateData) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return startOffset, nil
		}
		return startOffset, err
	}
	defer f.Close()
	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		return startOffset, err
	}
	br := bufio.NewReader(f)
	for {
		line, err := br.ReadBytes('\n')
		if err == io.EOF && len(line) == 0 {
			break
		}
		if len(line) == 0 {
			break
		}
		for len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			if err == io.EOF {
				break
			}
			continue
		}
		var raw map[string]any
		if json.Unmarshal(line, &raw) != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		if _, ok := raw["event_type"]; ok {
			handleEvent(raw, state)
			if err == io.EOF {
				break
			}
			continue
		}
		pl := defaultPlatform
		if p, ok := raw["platform"].(string); ok && p != "" {
			pl = p
		}
		var args map[string]any
		if a, ok := raw["action_args"].(map[string]any); ok {
			args = a
		}
		act := Action{
			RoundNum:   intFromAny(raw["round"]),
			Timestamp:  fmt.Sprint(raw["timestamp"]),
			Platform:   pl,
			AgentID:    intFromAny(raw["agent_id"]),
			AgentName:  fmt.Sprint(raw["agent_name"]),
			ActionType: fmt.Sprint(raw["action_type"]),
			ActionArgs: args,
			Result:     raw["result"],
			Success:    boolFromAny(raw["success"]),
		}
		if act.Timestamp == "" {
			act.Timestamp = rawTimeNow()
		}
		state.addAction(act)
		if err == io.EOF {
			break
		}
	}
	off, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return startOffset, err
	}
	return off, nil
}

func handleEvent(raw map[string]any, state *runStateData) {
	et := fmt.Sprint(raw["event_type"])
	switch et {
	case "simulation_end":
		pl := fmt.Sprint(raw["platform"])
		switch pl {
		case "twitter":
			state.TwitterCompleted = true
			state.TwitterRunning = false
		case "reddit":
			state.RedditCompleted = true
			state.RedditRunning = false
		default:
			state.TwitterCompleted = true
			state.RedditCompleted = true
			state.TwitterRunning = false
			state.RedditRunning = false
		}
		if state.allPlatformsDone() {
			state.RunnerStatus = "completed"
			now := rawTimeNow()
			state.CompletedAt = &now
		}
	case "round_end":
		rn := intFromAny(raw["round"])
		sh := intFromAny(raw["simulated_hours"])
		pl := fmt.Sprint(raw["platform"])
		if pl == "twitter" {
			if rn > state.TwitterCurrentRound {
				state.TwitterCurrentRound = rn
			}
			state.TwitterSimulatedHours = maxInt(state.TwitterSimulatedHours, sh)
		} else if pl == "reddit" {
			if rn > state.RedditCurrentRound {
				state.RedditCurrentRound = rn
			}
			state.RedditSimulatedHours = maxInt(state.RedditSimulatedHours, sh)
		}
		if rn > state.CurrentRound {
			state.CurrentRound = rn
		}
		state.SimulatedHours = maxInt(state.TwitterSimulatedHours, state.RedditSimulatedHours)
	}
}

func intFromAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case int64:
		return int(x)
	default:
		return 0
	}
}

func boolFromAny(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func collectAllActions(simDir, platformFilter string, agentID *int, roundNum *int) []Action {
	var paths []string
	tw := filepath.Join(simDir, "twitter", "actions.jsonl")
	rd := filepath.Join(simDir, "reddit", "actions.jsonl")
	if platformFilter == "" || platformFilter == "twitter" {
		paths = append(paths, tw)
	}
	if platformFilter == "" || platformFilter == "reddit" {
		paths = append(paths, rd)
	}
	legacy := filepath.Join(simDir, "actions.jsonl")
	var out []Action
	for _, p := range paths {
		out = append(out, scanActionsFile(p, platformFromPath(p, simDir), agentID, roundNum)...)
	}
	if len(out) == 0 {
		out = append(out, scanActionsFile(legacy, "", agentID, roundNum)...)
	}
	// newest first sort happens in caller
	return out
}

func platformFromPath(p, simDir string) string {
	if filepath.Dir(p) == filepath.Join(simDir, "twitter") {
		return "twitter"
	}
	if filepath.Dir(p) == filepath.Join(simDir, "reddit") {
		return "reddit"
	}
	return ""
}

func scanActionsFile(path, defaultPlatform string, agentID *int, roundNum *int) []Action {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 4*1024*1024)
	var actions []Action
	for sc.Scan() {
		line := sc.Bytes()
		var raw map[string]any
		if json.Unmarshal(line, &raw) != nil {
			continue
		}
		if _, ok := raw["event_type"]; ok {
			continue
		}
		pl := defaultPlatform
		if p, ok := raw["platform"].(string); ok && p != "" {
			pl = p
		}
		aid := intFromAny(raw["agent_id"])
		rn := intFromAny(raw["round"])
		if agentID != nil && aid != *agentID {
			continue
		}
		if roundNum != nil && rn != *roundNum {
			continue
		}
		var args map[string]any
		if a, ok := raw["action_args"].(map[string]any); ok {
			args = a
		}
		actions = append(actions, Action{
			RoundNum:   rn,
			Timestamp:  fmt.Sprint(raw["timestamp"]),
			Platform:   pl,
			AgentID:    aid,
			AgentName:  fmt.Sprint(raw["agent_name"]),
			ActionType: fmt.Sprint(raw["action_type"]),
			ActionArgs: args,
			Result:     raw["result"],
			Success:    boolFromAny(raw["success"]),
		})
	}
	return actions
}
