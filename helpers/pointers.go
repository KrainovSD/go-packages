package helpers

func IntFromPtr(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func StrFromPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func BoolFromPtr(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func IntToPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func StrToPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
