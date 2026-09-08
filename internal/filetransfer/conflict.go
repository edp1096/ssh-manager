package filetransfer

import (
	"context"
	"errors"
	"net/http"
	"os"
)

// A pending conflict is immutable while exposed through the jobs endpoint.
type Conflict struct {
	ID          string `json:"id"`
	Source      Entry  `json:"source"`
	Destination Entry  `json:"destination"`
}
type conflictError struct{ Conflict *Conflict }

func (e *conflictError) Error() string { return "Destination exists: " + e.Conflict.Destination.Path }

type resolution struct {
	ConflictID  string `json:"conflictId"`
	Action      string `json:"action"`
	Scope       string `json:"scope"`
	destination Entry
}

func fileEntry(p string, info os.FileInfo) Entry {
	return Entry{Name: info.Name(), Path: p, Size: info.Size(), Modified: info.ModTime(), Directory: info.IsDir()}
}
func (m *Manager) conflictAction(j *Job, dest string, info os.FileInfo) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if choice, ok := j.choices[dest]; ok && choice.destination.Size == info.Size() && choice.destination.Modified.Equal(info.ModTime()) {
		return choice.Action
	}
	if s := m.sessions[j.Session]; s != nil {
		if policy := s.batchPolicies[j.Batch]; policy != "" {
			return policy
		}
		if s.policy != "" {
			return s.policy
		}
	}
	return ""
}
func (m *Manager) waitConflict(ctx context.Context, j *Job, c *Conflict) error {
	m.mu.Lock()
	j.Status = "waiting"
	j.Conflict = c
	j.decision = make(chan resolution, 1)
	decisions := j.decision
	m.mu.Unlock()
	// transferRemote has returned and closed its network connection. Waiting for
	// the user must not consume a socket or inherit its idle timeout.
	var choice resolution
	select {
	case <-ctx.Done():
		return ctx.Err()
	case choice = <-decisions:
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	j.Conflict = nil
	j.decision = nil
	if choice.Action == "cancel" {
		for _, other := range m.jobs {
			if other.Session == j.Session && other.Batch == j.Batch {
				other.cancel()
			}
		}
		return context.Canceled
	}
	choice.destination = c.Destination
	j.choices[c.Destination.Path] = choice
	if s := m.sessions[j.Session]; s != nil {
		if choice.Scope == "session" {
			s.policy = choice.Action
		}
		if choice.Scope == "batch" {
			if s.batchPolicies == nil {
				s.batchPolicies = make(map[string]string)
			}
			// There can be at most 500 queued/history jobs. Discard policies
			// for batches with no retained jobs to bound session memory.
			for batch := range s.batchPolicies {
				found := false
				for _, job := range m.jobs {
					if job.Session == j.Session && job.Batch == batch {
						found = true
						break
					}
				}
				if !found {
					delete(s.batchPolicies, batch)
				}
			}
			s.batchPolicies[j.Batch] = choice.Action
		}
	}
	j.Status = "running"
	return nil
}
func (m *Manager) resolveConflict(w http.ResponseWriter, r *http.Request) {
	var choice resolution
	if err := decode(w, r, &choice); err != nil {
		respond(w, nil, err)
		return
	}
	if (choice.Action != "overwrite" && choice.Action != "skip" && choice.Action != "cancel") ||
		(choice.Scope != "file" && choice.Scope != "batch" && choice.Scope != "session") {
		respond(w, nil, errors.New("Invalid conflict choice"))
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.ID != r.PathValue("id") {
			continue
		}
		if j.Status != "waiting" || j.Conflict == nil || choice.ConflictID != j.Conflict.ID || j.decision == nil {
			break
		}
		select {
		case j.decision <- choice:
			// Reject duplicate responses before the worker consumes the choice.
			j.Status = "running"
			respond(w, map[string]bool{"ok": true}, nil)
			return
		default:
		}
	}
	respond(w, nil, errors.New("This conflict is no longer awaiting a decision"))
}
