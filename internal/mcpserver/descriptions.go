package mcpserver

import (
	"qLLM/internal/catalogdesc"
	"qLLM/internal/catalogidx"
)

// ToolDescriptions interpolates loaded catalog names into MCP tool descriptions.
type ToolDescriptions = catalogdesc.ToolDescriptions

// DescriptionsFor builds capped MCP descriptions from the loaded catalog only.
func DescriptionsFor(idx *catalogidx.Index) ToolDescriptions {
	return catalogdesc.DescriptionsFor(idx)
}

// DescriptionsForAllow builds capped MCP descriptions for an ACL projection.
func DescriptionsForAllow(idx *catalogidx.Index, acl *catalogidx.ACL) ToolDescriptions {
	return catalogdesc.DescriptionsForAllow(idx, acl)
}
