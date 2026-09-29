package cataloggen

import (
	"bytes"
	"fmt"
	"os"

	"qLLM/internal/protocol"

	"gopkg.in/yaml.v3"
)

// EncodeCatalog YAML for a catalog document.
func EncodeCatalog(c *protocol.Catalog) ([]byte, error) {
	if c.ProtocolVersion == "" {
		c.ProtocolVersion = protocol.ProtocolVersion
	}
	var node yaml.Node
	if err := node.Encode(c); err != nil {
		return nil, err
	}
	forceQuotedKeys(&node, map[string]bool{"protocolVersion": true})
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// EncodeResources YAML map suitable to paste under sources[].options.resources.
func EncodeResources(resources map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(resources); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

func WriteFile(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return protocol.NewError(protocol.ErrConfigError, fmt.Sprintf("write %s: %v", path, err), nil)
	}
	return nil
}

func forceQuotedKeys(n *yaml.Node, keys map[string]bool) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			forceQuotedKeys(c, keys)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if keys[k.Value] && v.Kind == yaml.ScalarNode {
				v.Style = yaml.DoubleQuotedStyle
				v.Tag = "!!str"
			}
			forceQuotedKeys(v, keys)
		}
	}
}
