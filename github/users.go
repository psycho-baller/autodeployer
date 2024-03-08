package gh

func getUsername() (string, error) {
	user, _, err := Globals.Client.Users.Get(Globals.Ctx, "")
	if err != nil {
			return "", err
	}

	return user.GetLogin(), nil
}