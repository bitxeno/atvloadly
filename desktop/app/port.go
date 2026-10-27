package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultServerPort = 5533

var errInvalidServerPort = errors.New("server port must be between 1 and 65535")

func validateServerPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%w: %d", errInvalidServerPort, port)
	}
	return nil
}

// desktopConfigPath returns the service config file in the app data directory.
func desktopConfigPath(dataDir string) string {
	return filepath.Join(dataDir, "config.yaml")
}

// ensureDesktopConfig creates the config with the desktop default port, or
// adds server.port when an existing config does not specify it. Existing
// settings and YAML comments are retained through the YAML node representation.
func ensureDesktopConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte("server:\n  port: " + strconv.Itoa(defaultServerPort) + "\n")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid config %s: top level must be a mapping", path)
	}

	server := mappingValue(root, "server")
	if server == nil {
		server = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		appendMappingPair(root, "server", server)
	}
	if server.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid config %s: server must be a mapping", path)
	}

	port := mappingValue(server, "port")
	if port != nil {
		if port.Kind != yaml.ScalarNode || port.Tag != "!!int" {
			return fmt.Errorf("invalid server.port in %s: must be an integer", path)
		}
		value, err := strconv.Atoi(strings.TrimSpace(port.Value))
		if err != nil {
			return fmt.Errorf("invalid server.port in %s: %w", path, err)
		}
		if err := validateServerPort(value); err != nil {
			return fmt.Errorf("invalid server.port in %s: %w", path, err)
		}
		return nil
	}

	appendMappingPair(server, "port", &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: strconv.Itoa(defaultServerPort),
	})
	updated, err := yaml.Marshal(&document)
	if err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	if err := os.WriteFile(path, updated, 0o600); err != nil {
		return fmt.Errorf("save config %s: %w", path, err)
	}
	return nil
}

// setServerPort writes a validated port into the existing server.port field,
// preserving other YAML configuration and comments.
func setServerPort(path string, port int) error {
	if err := validateServerPort(port); err != nil {
		return err
	}

	if err := ensureDesktopConfig(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("invalid config %s: top level must be a mapping", path)
	}
	server := mappingValue(document.Content[0], "server")
	if server == nil || server.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid config %s: server must be a mapping", path)
	}
	portNode := mappingValue(server, "port")
	if portNode == nil {
		appendMappingPair(server, "port", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(port)})
	} else {
		portNode.Kind = yaml.ScalarNode
		portNode.Tag = "!!int"
		portNode.Value = strconv.Itoa(port)
		portNode.Style = 0
	}

	updated, err := yaml.Marshal(&document)
	if err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	if err := os.WriteFile(path, updated, 0o600); err != nil {
		return fmt.Errorf("save config %s: %w", path, err)
	}
	return nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func appendMappingPair(mapping *yaml.Node, key string, value *yaml.Node) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		value,
	)
}
