package models

import "context"

const maxSessionLogs = 1000

func appendSessionLog(logs []LogEntry, entry LogEntry) []LogEntry {
	if len(logs) >= maxSessionLogs {
		copy(logs, logs[len(logs)-maxSessionLogs+1:])
		logs = logs[:maxSessionLogs-1]
	}
	return append(logs, entry)
}

func (s *AutoRegisterSession) RunContext() context.Context {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Context == nil {
		s.Context, s.CancelFunc = context.WithCancel(context.Background())
	}
	if s.OwnerToken == "" {
		s.OwnerToken = s.Token
	}
	return s.Context
}

func (s *AutoRegisterSession) Finish(status string, result interface{}) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Active {
		s.Active, s.Status, s.Result = false, status, result
	}
	if s.CancelFunc != nil {
		s.CancelFunc()
	}
}

func (s *AutoEventSession) RunContext() context.Context {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Context == nil {
		s.Context, s.CancelFunc = context.WithCancel(context.Background())
	}
	if s.OwnerToken == "" {
		s.OwnerToken = s.Token
	}
	return s.Context
}

func (s *AutoEventSession) Finish(status string, result interface{}) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Active {
		s.Active, s.Status, s.Result = false, status, result
	}
	if s.CancelFunc != nil {
		s.CancelFunc()
	}
}
