package bitx

import "testing"

func TestExample(t *testing.T) {
	c := NewClient("test", "test")
	if c == nil {
		t.Errorf("Expected valid client, got: %v", c)
	}
}
