package cmd

func getOrDefault(key, def string, data map[string]string) (result string) {
	var ok bool
	if result, ok = data[key]; !ok {
		result = def
	}
	return
}

func getReplacement(key string, data map[string]string) (result string) {
	return getOrDefault(key, key, data)
}
