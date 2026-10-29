package helpers

import (
	"testing"
)

func TestSafeWork(t *testing.T) {
	t.Run("recover", func(t *testing.T) {
		var err = SafeWork(func() error {
			var slice []string = make([]string, 0, 5)
			var _ = slice[2]
			return nil
		})
		if err == nil {
			t.Errorf("expected error")
		}
	})
}
