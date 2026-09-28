package quota

var findActiveServersFn func(string) ([]ActiveServer, error)

// FindActiveServers locates running language servers for a target profile (or all profiles).
func FindActiveServers(targetProf string) ([]ActiveServer, error) {
	if findActiveServersFn != nil {
		return findActiveServersFn(targetProf)
	}
	return findActiveServersOS(targetProf)
}

// SetTestHooks injects custom discovery logic for hermetic testing and returns a cleanup func.
func SetTestHooks(fn func(string) ([]ActiveServer, error)) func() {
	old := findActiveServersFn
	findActiveServersFn = fn
	return func() {
		findActiveServersFn = old
	}
}
