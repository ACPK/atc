package main

import "net/url"

type URLFlag *url.URL

func (u *URLFlag) UnmarshalFlag(value string) error {
	parsedURL, err := url.Parse(value)
	if err != nil {
		return err
	}

	*u = URLFlag(parsedURL)

	return nil
}
