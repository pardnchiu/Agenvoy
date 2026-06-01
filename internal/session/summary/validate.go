package summary

func IsValid(dic map[string]any) bool {
	newKeys := []string{"key_decisions", "past_discussions", "current_discussion"}
	if match(dic, newKeys) >= 2 {
		return true
	}
	legacyKeys := []string{"core_discussion", "discussion_log", "confirmed_needs", "current_conclusion"}
	return match(dic, legacyKeys) >= 2
}

func match(dic map[string]any, keys []string) int {
	num := 0
	for _, key := range keys {
		if _, ok := dic[key]; ok {
			num++
		}
	}
	return num
}
