package probe

type HTTPProbeError struct {
	Engine   string
	Stage    string
	ExitCode int
	Detail   string
	Proto    string
}

func (e *HTTPProbeError) Error() string {
	if e == nil {
		return ""
	}
	return e.Detail
}
