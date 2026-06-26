package mcp

// OkOutput is the response shape for tools whose only result is success/failure.
type OkOutput struct {
	Ok bool `json:"ok"`
}
