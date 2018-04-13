package streaming

import "testing"

func TestWithUpdateCallback(t *testing.T) {
	var c Conn
	WithUpdateCallback(func(u Update) {})(&c)
	if c.updateCallback == nil {
		t.Errorf("Expected non-nil")
	}
}
