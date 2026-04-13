package utils

// C2CChatID returns a stable chat id for one-to-one chats.
func C2CChatID(a, b string) string {
	if a <= b {
		return "c2c:" + a + ":" + b
	}
	return "c2c:" + b + ":" + a
}
