package mcp

// noopLogger swallows every log line. Used as the default logger so nil
// checks are unnecessary throughout the package.
type noopLogger struct{}

func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}

// NewNoopLogger returns a Logger that discards all output.
func NewNoopLogger() Logger { return noopLogger{} }
