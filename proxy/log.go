package proxy

type NopWriter struct{}

func (NopWriter) Write(p []byte) (int, error) {
	return len(p), nil
}
