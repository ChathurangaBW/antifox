package server

import "antifox/agent"

// sameLLMEndpoint reports whether two provider/base URL pairs resolve to the same
// endpoint after the normalization the runtime applies (agent.ConfigFrom: default
// base URL per provider, trailing slash and endpoint suffix trimmed). Ported from
// ARTEX (#142).
//
// A saved API key is never shown to anyone, including admins. These checks keep
// that promise: a saved key is only ever sent to the endpoint it was saved for.
// Before them, a request could name its own base URL and receive the saved key,
// through the connection test, the global LLM save, or a profile edit.
func sameLLMEndpoint(provA, baseA, provB, baseB string) bool {
	a := agent.ConfigFrom(provA, "", baseA, "", "")
	b := agent.ConfigFrom(provB, "", baseB, "", "")
	return a.Format == b.Format && a.BaseURL == b.BaseURL
}

// errKeyEndpointChanged is returned when a request would reuse a saved key for a
// different provider or base URL.
const errKeyEndpointChanged = "the saved API key is only sent to the endpoint it was saved for: enter the API key again to use a different provider or base URL"
