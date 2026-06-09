package combine

func matchRoleByPassword(passwords map[string]struct {
	Hash string
	Role Role
}, password string) (Role, bool) {
	hash := sha256Hex(password)
	for _, item := range passwords {
		if item.Hash == hash {
			return item.Role, true
		}
	}
	return "", false
}
