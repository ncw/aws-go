package aws

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"net/http"
)

// RestClient is the underlying client for REST-JSON and REST-XML APIs.
type RestClient struct {
	Context    Context
	Client     *http.Client
	Endpoint   string
	APIVersion string
}

// Do sends an HTTP request and returns an HTTP response, following policy
// (e.g. redirects, cookies, auth) as configured on the client.
func (c *RestClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "aws-go")
	if err := c.Context.sign(req); err != nil {
		return nil, err
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var err restError
		switch resp.Header.Get("Content-Type") {
		case "application/json":
			if err := json.NewDecoder(resp.Body).Decode(&err); err != nil {
				return nil, err
			}
			return nil, err.Err()
		case "application/xml", "text/xml":
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			body := bytes.NewReader(bodyBytes)

			// AWS XML error documents can have a couple of different formats.
			// Try each before returning a decode error.
			var wrappedErr restErrorResponse
			if err := xml.NewDecoder(body).Decode(&wrappedErr); err == nil {
				return nil, wrappedErr.Error.Err()
			}
			body.Seek(0, 0)
			if err := xml.NewDecoder(body).Decode(&err); err != nil {
				return nil, err
			}
			return nil, err.Err()
		default:
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return nil, errors.New(string(b))
		}
	}

	return resp, nil
}

type restErrorResponse struct {
	XMLName xml.Name `xml:"ErrorResponse",json:"-"`
	Error   restError
}

type restError struct {
	XMLName    xml.Name `xml:"Error",json:"-"`
	Code       string
	BucketName string
	Message    string
	RequestID  string
	HostID     string
}

func (e restError) Err() error {
	return APIError{
		Code:      e.Code,
		Message:   e.Message,
		RequestID: e.RequestID,
		HostID:    e.HostID,
		Specifics: map[string]string{
			"BucketName": e.BucketName,
		},
	}
}
