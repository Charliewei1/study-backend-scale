package middleware

func skipRequestInstrumentation(path string) bool {
	// Scrapes and health probes are infrastructure traffic. Counting or logging
	// them as user requests pollutes RED metrics and request logs.
	switch path {
	case "/metrics", "/healthz", "/readyz":
		return true
	default:
		return false
	}
}
