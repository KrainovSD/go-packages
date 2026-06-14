package helpers

func FilterMutable[T any](ptr *[]T, condition func(*T) bool) {
	n := 0
	var zero T
	var slice = *ptr
	for i := range slice {
		if condition(&slice[i]) {
			if n != i {
				slice[n] = slice[i]
				slice[i] = zero
			}
			n++
		} else {
			slice[i] = zero
		}
	}
	*ptr = slice[:n:n]
}

func ReverseSlice[T any](slice []T) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}
