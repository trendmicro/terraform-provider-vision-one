package trendmicro

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"terraform-provider-vision-one/pkg/dto"
)

// HostURL - Default Hashicups URL

// Client -
type Client struct {
	HostURL         string
	HTTPClient      *http.Client
	BearerToken     string
	TMUserAgent     string
	ProviderVersion string
}

// AuthResponse -
type AuthResponse struct {
	Status int
}

const (
	StatusVisionOneInnerError = 491

	TMUserAgent = "TMXDRContainerTerraform"

	// global User-Agent header for all requests
	UserAgentHeader = "TMV1-Terraform-Provider"
)

// NewClient -
func NewClient(host, token *string, version string) (*Client, error) {
	c := Client{
		HTTPClient:      &http.Client{Timeout: 10 * time.Second},
		HostURL:         *host,
		BearerToken:     *token,
		TMUserAgent:     TMUserAgent,
		ProviderVersion: version,
	}

	_, err := c.Auth()
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Client) DoRequest(req *http.Request) (body []byte, err error) {
	req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	req.Header.Set("HOST", c.HostURL)
	req.Header.Set("User-Agent", UserAgentHeader+"/"+c.ProviderVersion)
	req.Header.Set("x-tm-user-agent", c.TMUserAgent+"/"+c.ProviderVersion)

	fmt.Printf("Sending HTTP Request %v", req)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	fmt.Printf("HTTP Response %v", res)
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return body, nil
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, StatusVisionOneInnerError:
		var out bytes.Buffer
		err = json.Indent(&out, body, "", "  ")
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("\n%w \nTrace id: %s", errors.New(out.String()), res.Header.Get("x-trace-id"))
	default:
		return nil, fmt.Errorf("%w trace id: %s", dto.ErrorInternal, res.Header.Get("x-trace-id"))
	}
}

func (c *Client) DoRequestWithFullResponse(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	req.Header.Set("HOST", c.HostURL)
	req.Header.Set("User-Agent", UserAgentHeader+"/"+c.ProviderVersion)
	req.Header.Set("x-tm-user-agent", c.TMUserAgent+"/"+c.ProviderVersion)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	switch res.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return res, nil
	case http.StatusNotFound, http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, StatusVisionOneInnerError:
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}

		var out bytes.Buffer
		err = json.Indent(&out, body, "", "  ")
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("\n%w \nTrace id: %s", errors.New(out.String()), res.Header.Get("x-trace-id"))
	default:
		return nil, fmt.Errorf("%w \nTrace id: %s", dto.ErrorInternal, res.Header.Get("x-trace-id"))
	}
}

// Auth - Authenticate the client with the Trend Micro Vision One API Secret Token and validate connectivity
func (c *Client) Auth() (*AuthResponse, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3.0/healthcheck/connectivity", c.HostURL), http.NoBody)
	if err != nil {
		return nil, err
	}

	body, err := c.DoRequest(req)
	if err != nil {
		return nil, err
	}

	bodyJSON := struct {
		Status string `json:"status"`
	}{}

	err = json.Unmarshal(body, &bodyJSON)
	if err != nil {
		return nil, err
	}

	if bodyJSON.Status != "available" {
		return nil, fmt.Errorf("authentication failed with status: %s", bodyJSON.Status)
	}

	return nil, nil
}
