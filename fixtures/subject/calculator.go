package subject

func Add(left, right int) int {
	return left + right
}

func Subtract(left, right int) int {
	return left - right
}

func Normalize(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func Describe(value int) string {
	return "value=" + string(rune('0'+Normalize(value)))
}
