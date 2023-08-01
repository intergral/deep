/*
 * Copyright (C) 2023  Intergral GmbH
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package util

import (
	"fmt"
	"github.com/intergral/deep/pkg/deeppb"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/golang/protobuf/jsonpb" //nolint:all
	"github.com/golang/protobuf/proto"  //nolint:all
	"github.com/klauspost/compress/gzhttp"
)

const (
	orgIDHeader = TenantIDHeaderName

	QuerySnapshotsEndpoint = "/api/snapshots"

	acceptHeader        = "Accept"
	applicationProtobuf = "application/protobuf"
	applicationJSON     = "application/json"
)

// Client is client to the Deep API.
type Client struct {
	BaseURL  string
	TenantID string
	client   *http.Client
}

func NewClient(baseURL, tenantID string) *Client {
	return &Client{
		BaseURL:  baseURL,
		TenantID: tenantID,
		client:   http.DefaultClient,
	}
}

func NewClientWithCompression(baseURL, tenantID string) *Client {
	c := NewClient(baseURL, tenantID)
	c.WithTransport(gzhttp.Transport(http.DefaultTransport))
	return c
}

func (c *Client) WithTransport(t http.RoundTripper) {
	c.client.Transport = t
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

func (c *Client) getFor(url string, m proto.Message) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if len(c.TenantID) > 0 {
		req.Header.Set(orgIDHeader, c.TenantID)
	}

	marshallingFormat := applicationJSON
	if strings.Contains(url, QuerySnapshotsEndpoint) {
		marshallingFormat = applicationProtobuf
	}
	// Set 'Accept' header to 'application/protobuf'.
	// This is required for the /api/snapshots endpoint to return a protobuf response.
	// JSON lost backwards compatibility with the upgrade to `opentelemetry-proto` v0.18.0.
	req.Header.Set(acceptHeader, marshallingFormat)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error searching deep for tag %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		body, _ := io.ReadAll(resp.Body)
		return resp, fmt.Errorf("GET request to %s failed with response: %d body: %s", req.URL.String(), resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch marshallingFormat {
	case applicationJSON:
		if err = jsonpb.UnmarshalString(string(body), m); err != nil {
			return resp, fmt.Errorf("error decoding %T json, err: %v body: %s", m, err, string(body))
		}
	case applicationProtobuf:

		if err = proto.Unmarshal(body, m); err != nil {
			return nil, fmt.Errorf("error decoding %T proto, err: %w body: %s", m, err, string(body))
		}
	}

	return resp, nil
}

func (c *Client) SearchTags() (*deeppb.SearchTagsResponse, error) {
	m := &deeppb.SearchTagsResponse{}
	_, err := c.getFor(c.BaseURL+"/api/search/tags", m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (c *Client) SearchTagValues(key string) (*deeppb.SearchTagValuesResponse, error) {
	m := &deeppb.SearchTagValuesResponse{}
	_, err := c.getFor(c.BaseURL+"/api/search/tag/"+key+"/values", m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// Search Deep. tags must be in logfmt format, that is "key1=value1 key2=value2"
func (c *Client) Search(tags string) (*deeppb.SearchResponse, error) {
	m := &deeppb.SearchResponse{}
	_, err := c.getFor(c.BaseURL+"/api/search?tags="+url.QueryEscape(tags), m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// SearchWithRange calls the /api/search endpoint. tags is expected to be in logfmt format and start/end are unix
// epoch timestamps in seconds.
func (c *Client) SearchWithRange(tags string, start int64, end int64) (*deeppb.SearchResponse, error) {
	m := &deeppb.SearchResponse{}
	_, err := c.getFor(c.BaseURL+"/api/search?tags="+url.QueryEscape(tags)+"&start="+strconv.FormatInt(start, 10)+"&end="+strconv.FormatInt(end, 10), m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func (c *Client) QuerySnapshot(id string) (*deep_tp.Snapshot, error) {
	m := &deep_tp.Snapshot{}
	resp, err := c.getFor(c.BaseURL+QuerySnapshotsEndpoint+"/"+id, m)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}

	return m, nil
}

func (c *Client) SearchDeepQL(query string) (*deeppb.SearchResponse, error) {
	m := &deeppb.SearchResponse{}
	_, err := c.getFor(c.BaseURL+"/api/search?q="+url.QueryEscape(query), m)
	if err != nil {
		return nil, err
	}

	return m, nil
}
