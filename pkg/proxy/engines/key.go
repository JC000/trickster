/*
 * Copyright 2018 Comcast Cable Communications Management, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package engines

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/tricksterproxy/trickster/pkg/proxy/errors"
	"github.com/tricksterproxy/trickster/pkg/proxy/headers"
	"github.com/tricksterproxy/trickster/pkg/proxy/request"
	"github.com/tricksterproxy/trickster/pkg/util/md5"
)

var methodsWithBody = map[string]bool{http.MethodPut: true, http.MethodPost: true, http.MethodPatch: true}

// DeriveCacheKey calculates a query-specific keyname based on the prometheus query in the user request
func (pr *proxyRequest) DeriveCacheKey(templateURL *url.URL, extra string) string {

	rsc := request.GetResources(pr.Request)
	pc := rsc.PathConfig

	if pc == nil {
		return md5.Checksum(pr.URL.Path + extra)
	}

	var params url.Values
	r := pr.Request

	if pr.upstreamRequest != nil {
		r = pr.upstreamRequest
		if r.URL == nil {
			r.URL = pr.URL
			params = pr.URL.Query()
		}
	}

	var b []byte
	if r.Method == http.MethodPost {
		b, _ = ioutil.ReadAll(r.Body)
		r.ParseForm()
		params = r.PostForm
		r.Body = ioutil.NopCloser(bytes.NewReader(b))
	} else if templateURL != nil {
		params = templateURL.Query()
	} else if r.URL != nil {
		params = r.URL.Query()
	}

	if pc.KeyHasher != nil && len(pc.KeyHasher) == 1 {
		var k string
		k, r.Body = pc.KeyHasher[0](r.URL.Path, params, r.Header, r.Body, extra)
		return k
	}

	vals := make([]string, 0, (len(pc.CacheKeyParams) + len(pc.CacheKeyHeaders) + len(pc.CacheKeyFormFields)*2))

	if v := r.Header.Get(headers.NameAuthorization); v != "" {
		vals = append(vals, fmt.Sprintf("%s.%s.", headers.NameAuthorization, v))
	}

	// Append the http method to the slice for creating the derived cache key
	vals = append(vals, fmt.Sprintf("%s.%s.", "method", r.Method))

	if len(pc.CacheKeyParams) == 1 && pc.CacheKeyParams[0] == "*" {
		for p := range params {
			vals = append(vals, fmt.Sprintf("%s.%s.", p, params.Get(p)))
		}
	} else {
		for _, p := range pc.CacheKeyParams {
			if v := params.Get(p); v != "" {
				vals = append(vals, fmt.Sprintf("%s.%s.", p, v))
			}
		}
	}

	for _, p := range pc.CacheKeyHeaders {
		if v := r.Header.Get(p); v != "" {
			vals = append(vals, fmt.Sprintf("%s.%s.", p, v))
		}
	}

	if _, ok := methodsWithBody[r.Method]; ok && pc.CacheKeyFormFields != nil && len(pc.CacheKeyFormFields) > 0 {
		ct := r.Header.Get(headers.NameContentType)
		if ct == headers.ValueXFormURLEncoded ||
			strings.HasPrefix(ct, headers.ValueMultipartFormData) || ct == headers.ValueApplicationJSON {
			if strings.HasPrefix(ct, headers.ValueMultipartFormData) {
				pr.ParseMultipartForm(1024 * 1024)
			} else if ct == headers.ValueApplicationJSON {
				var document map[string]interface{}
				err := json.Unmarshal(b, &document)
				if err == nil {
					for _, f := range pc.CacheKeyFormFields {
						v, err := deepSearch(document, f)
						if err == nil {
							if pr.Form == nil {
								pr.Form = url.Values{}
							}
							pr.Form.Set(f, v)
						}
					}
				}
			}
			pr.Body = ioutil.NopCloser(bytes.NewReader(b))
		}

		for _, f := range pc.CacheKeyFormFields {
			if _, ok := pr.Form[f]; ok {
				if v := pr.FormValue(f); v != "" {
					vals = append(vals, fmt.Sprintf("%s.%s.", f, v))
				}
			}
		}
	}

	sort.Strings(vals)
	return md5.Checksum(pr.URL.Path + "." + strings.Join(vals, "") + extra)
}

func deepSearch(document map[string]interface{}, key string) (string, error) {

	if key == "" {
		return "", fmt.Errorf("invalid key name: %s", key)
	}
	parts := strings.Split(key, "/")
	m := document
	l := len(parts) - 1
	for i, p := range parts {
		v, ok := m[p]
		if !ok {
			return "", errors.CouldNotFindKey(key)
		}
		if l != i {
			m, ok = v.(map[string]interface{})
			if !ok {
				return "", errors.CouldNotFindKey(key)
			}
			continue
		}

		if s, ok := v.(string); ok {
			return s, nil
		}

		if i, ok := v.(float64); ok {
			return strconv.FormatFloat(i, 'f', 4, 64), nil
		}

		if b, ok := v.(bool); ok {
			return fmt.Sprintf("%t", b), nil
		}

	}
	return "", errors.CouldNotFindKey(key)
}