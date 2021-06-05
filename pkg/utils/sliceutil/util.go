package sliceutil

func ContainString(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}

func RemoveString(origin []string, str string) []string {
	for i, v := range origin {
		if v == str {
			return append(origin[:i], origin[i+1:]...)
		}
	}
	return origin
}
