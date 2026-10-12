package main

// startupError records a failed startup step together with the exact log attributes main emits.
type startupError struct {
	step  string
	attrs []any
}

func (e *startupError) Error() string { return e.step }

func stepFailed(step string, err error) error {
	return &startupError{step: step, attrs: []any{"error", err}}
}
