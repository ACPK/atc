package github

import (
	"net/http"

	"github.com/concourse/atc/auth"
	"github.com/hashicorp/go-multierror"
	"github.com/pivotal-golang/lager"
)

type verifierBasket struct {
	verifiers []auth.Verifier
}

func NewVerifierBasket(verifiers ...auth.Verifier) auth.Verifier {
	return &verifierBasket{verifiers: verifiers}
}

func (vb *verifierBasket) Verify(logger lager.Logger, client *http.Client) (bool, error) {
	var errors error

	for _, verifier := range vb.verifiers {
		verified, err := verifier.Verify(logger, client)
		if err != nil {
			errors = multierror.Append(errors, err)
			continue
		}
		if verified {
			return true, nil
		}
	}

	return false, errors
}
