package github

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/pivotal-golang/lager"
)

type UserVerifier struct {
	users        []string
	gitHubClient Client
}

func NewUserVerifier(
	users []string,
	gitHubClient Client,
) auth.Verifier {
	return &UserVerifier{
		users:        users,
		gitHubClient: gitHubClient,
	}
}

func (verifier *UserVerifier) Verify(logger lager.Logger, httpClient *http.Client) (bool, error) {
	currentUser, err := verifier.gitHubClient.CurrentUser(httpClient)
	if err != nil {
		logger.Error("failed-to-get-current-user", err)
		return false, err
	}

	for _, user := range verifier.users {
		if user == currentUser {
			return true, nil
		}
	}

	logger.Info("not-validated-user", lager.Data{
		"have": currentUser,
		"want": verifier.users,
	})

	return false, nil
}
