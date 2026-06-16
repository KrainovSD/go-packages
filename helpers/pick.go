package helpers

func FirstNonEmptyStringPtr(vals ...*string) *string {
	for _, v := range vals {
		if v != nil && *v != "" {
			return v
		}
	}
	return nil
}

func FirstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func FirstNonEmptyIntPtr(vals ...*int) *int {
	for _, v := range vals {
		if v != nil && *v != 0 {
			return v
		}
	}
	return nil
}

func FirstNonEmptyInt(vals ...int) int {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}
