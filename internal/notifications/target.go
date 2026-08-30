package notifications

import (
	"fmt"
	"net/url"
)

type NotificationTarget struct {
	URL           string
	NotifySuccess bool
}

func (nt *NotificationTarget) ValidateURL() error {
	u, err := url.ParseRequestURI(nt.URL)
	if err != nil {
		return err
	}

	if !((u.Scheme == "http" || u.Scheme == "https") && u.Host != "") {
		return fmt.Errorf("invalid URL scheme: %s or host (%s) is empty", u.Scheme, u.Host)
	}
	return nil
}
