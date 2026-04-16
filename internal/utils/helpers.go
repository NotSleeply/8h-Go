package utils

func C2CChatID(a, b string) string {
	if a <= b {
		return "c2c:" + a + ":" + b
	}
	return "c2c:" + b + ":" + a
}
