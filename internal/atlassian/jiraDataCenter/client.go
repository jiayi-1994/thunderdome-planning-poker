package jiradatacenter

import (
	"fmt"
	"net/http"
	"time"

	jira "github.com/andygrunwald/go-jira/v2/onpremise"
)

// New creates a new JIRA client
func New(config Config) (*Client, error) {
	var httpClient *http.Client
	switch config.AuthMethod {
	case "basic":
		tp := jira.BasicAuthTransport{Username: config.ClientMail, Password: config.AccessToken}
		httpClient = tp.Client()
	case "", "pat":
		tp := jira.BearerAuthTransport{Token: config.AccessToken}
		httpClient = tp.Client()
	default:
		return nil, fmt.Errorf("unsupported Jira authentication method")
	}
	httpClient.Timeout = 10 * time.Second
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	instance, err := jira.NewClient(config.InstanceHost, httpClient)
	if err != nil {
		return nil, err
	}
	return &Client{
		instance: instance,
	}, nil
}
