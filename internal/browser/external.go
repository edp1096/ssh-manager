package browser

const RepositoryURL = "https://github.com/edp1096/ssh-manager"

func OpenRepository() error {
	return openExternalURL(RepositoryURL)
}
