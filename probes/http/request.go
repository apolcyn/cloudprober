// Copyright 2019-2023 The Cloudprober Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloudprober/cloudprober/common/httputils"
	"github.com/cloudprober/cloudprober/logger"
	"github.com/cloudprober/cloudprober/targets/endpoint"
	"golang.org/x/oauth2"
)

const relURLLabel = "relative_url"

func hostWithPort(host string, port int) string {
	if port == 0 {
		return host
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// hostHeaderForTarget computes request's Host header for a target.
//   - If host header is set in the probe, it overrides everything else.
//   - If target's fqdn is provided in its labels, use that along with the
//     given port.
//   - Finally, use target's name with port.
func hostHeaderForTarget(target endpoint.Endpoint, probeHostHeader string, port int) string {
	if probeHostHeader != "" {
		return probeHostHeader
	}

	if target.Labels["fqdn"] != "" {
		return hostWithPort(target.Labels["fqdn"], port)
	}

	return hostWithPort(target.Name, port)
}

func urlHostForTarget(target endpoint.Endpoint) string {
	if target.Labels["fqdn"] != "" {
		return target.Labels["fqdn"]
	}

	return target.Name
}

func relURLForTarget(target endpoint.Endpoint, probeURL string) string {
	if probeURL != "" {
		return probeURL
	}

	if target.Labels[relURLLabel] != "" {
		return target.Labels[relURLLabel]
	}

	return ""
}

func (p *Probe) httpRequestForTarget(target endpoint.Endpoint) *http.Request {
	// Prepare HTTP.Request for Client.Do
	port := int(p.c.GetPort())
	// If port is not configured explicitly, use target's port if available.
	if port == 0 {
		port = target.Port
	}

	urlHost := urlHostForTarget(target)
	ipForLabel := ""

	resolveFirst := false
	if p.c.ResolveFirst != nil {
		resolveFirst = p.c.GetResolveFirst()
	} else {
		resolveFirst = target.IP != nil
	}
	if resolveFirst {
		ip, err := target.Resolve(p.opts.IPVersion, p.opts.Targets)
		if err != nil {
			p.l.Error("target: ", target.Name, ", resolve error: ", err.Error())
			return nil
		}

		ipStr := ip.String()
		urlHost, ipForLabel = ipStr, ipStr
	}

	for _, al := range p.opts.AdditionalLabels {
		al.UpdateForTarget(target, ipForLabel, port)
	}

	// Put square brackets around literal IPv6 hosts. This is the same logic as
	// net.JoinHostPort, but we cannot use net.JoinHostPort as it works only for
	// non default ports.
	if strings.IndexByte(urlHost, ':') >= 0 {
		urlHost = "[" + urlHost + "]"
	}

	url := fmt.Sprintf("%s://%s%s", p.protocol, hostWithPort(urlHost, port), relURLForTarget(target, p.url))

	req, err := httputils.NewRequest(p.method, url, p.requestBody)
	if err != nil {
		p.l.Error("target: ", target.Name, ", error creating HTTP request: ", err.Error())
		return nil
	}

	var probeHostHeader string
	for _, header := range p.c.GetHeaders() {
		if header.GetName() == "Host" {
			probeHostHeader = header.GetValue()
			continue
		}
		req.Header.Set(header.GetName(), header.GetValue())
	}

	// Host header is set by http.NewRequest based on the URL, update it based
	// on various conditions.
	req.Host = hostHeaderForTarget(target, probeHostHeader, port)

	if p.c.GetUserAgent() != "" {
		req.Header.Set("User-Agent", p.c.GetUserAgent())
	}

	return req
}

func getToken(ts oauth2.TokenSource, l *logger.Logger) (string, error) {
	tok, err := ts.Token()
	if err != nil {
		return "", err
	}
	l.Debug("Got OAuth token, len: ", strconv.FormatInt(int64(len(tok.AccessToken)), 10), ", expirationTime: ", tok.Expiry.String())

	if tok.AccessToken != "" {
		return tok.AccessToken, nil
	}

	idToken, ok := tok.Extra("id_token").(string)
	if ok {
		return idToken, nil
	}

	return "", fmt.Errorf("got unknown token: %v", tok)
}

func (p *Probe) prepareRequest(req *http.Request) *http.Request {
	// We clone the request for the cases where we modify the request:
	//   -- if request body is large (buffered), each request gets its own Body
	//      as HTTP transport reads body in a streaming fashion, and we can't
	//      share it across multiple requests.
	//   -- if OAuth token is used, each request gets its own Authorization
	//      header.
	if p.oauthTS == nil && !p.requestBody.Buffered() {
		return req
	}

	req = req.Clone(req.Context())

	if p.oauthTS != nil {
		tok, err := getToken(p.oauthTS, p.l)
		// Note: We don't terminate the request if there is an error in getting
		// token. That is to avoid complicating the flow, and to make sure that
		// OAuth refresh failures show in probe failures.
		if err != nil {
			p.l.Error("Error getting OAuth token: ", err.Error())
			tok = "<token-missing>"
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	if p.requestBody.Buffered() {
		req.Body = p.requestBody.Reader()
	}

	return req
}
