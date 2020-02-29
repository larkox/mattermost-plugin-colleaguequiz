package main

func contains(slice []string, element string) bool {
	for _, v := range slice {
		if v == element {
			return true
		}
	}
	return false
}

func (p *Plugin) GetUserName(userID string) string {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return ""
	}

	return user.Username
}
