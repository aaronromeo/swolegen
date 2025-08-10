package workout

// EstimateSets returns a rough set count given duration and average per-set time.
func EstimateSets(durationMinutes int, avgSecondsPerSet int) int {
	if avgSecondsPerSet <= 0 {
		avgSecondsPerSet = 90
	}
	if durationMinutes <= 0 {
		return 0
	}
	sec := durationMinutes * 60
	return sec / avgSecondsPerSet
}
