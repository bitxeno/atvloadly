package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

const (
	defaultServerPort = 5533
	// loopbackListenAddr keeps the desktop service reachable from this machine
	// only; lanListenAddr exposes it to the local network.
	loopbackListenAddr = "127.0.0.1"
	lanListenAddr      = "0.0.0.0"
)

var errInvalidServerPort = errors.New("server port must be between 1 and 65535")

// desktopSettings are the service settings the desktop shell edits. They map
// directly to the existing server section of config.yaml.
type desktopSettings struct {
	Port       int
	ListenAddr string
}

// allowLAN reports whether the service listens beyond the loopback address.
func (s desktopSettings) allowLAN() bool {
	return s.ListenAddr != "" && s.ListenAddr != loopbackListenAddr
}

func validateServerPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%w: %d", errInvalidServerPort, port)
	}
	return nil
}

func validateListenAddr(addr string) error {
	if addr != loopbackListenAddr && addr != lanListenAddr {
		return fmt.Errorf("server listen address must be %s or %s", loopbackListenAddr, lanListenAddr)
	}
	return nil
}

// desktopConfigPath returns the service config file in the app data directory.
func desktopConfigPath(dataDir string) string {
	return filepath.Join(dataDir, "config.yaml")
}

// ensureDesktopConfig creates the config with the desktop defaults, or fills in
// server.port and server.listen_addr when an existing config omits them. Other
// settings and YAML comments are retained through the YAML node representation.
func ensureDesktopConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		data = []byte("server:\n  port: " + strconv.Itoa(defaultServerPort) + "\n  listen_addr: " + loopbackListenAddr + "\n")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	document, root, err := parseDesktopConfig(path, data)
	if err != nil {
		return err
	}
	server, err := ensureMapping(root, "server", path)
	if err != nil {
		return err
	}

	changed := false
	if mappingValue(server, "port") == nil {
		appendMappingPair(server, "port", integerNode(defaultServerPort))
		changed = true
	}
	if mappingValue(server, "listen_addr") == nil {
		appendMappingPair(server, "listen_addr", stringNode(loopbackListenAddr))
		changed = true
	}
	if !changed {
		return nil
	}
	return writeDesktopConfig(path, document)
}

// loadDesktopSettings reads server.port and server.listen_addr, creating the
// desktop defaults on first launch.
func loadDesktopSettings(path string) (desktopSettings, error) {
	if err := ensureDesktopConfig(path); err != nil {
		return desktopSettings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return desktopSettings{}, fmt.Errorf("read config: %w", err)
	}
	_, root, err := parseDesktopConfig(path, data)
	if err != nil {
		return desktopSettings{}, err
	}

	settings := desktopSettings{Port: defaultServerPort, ListenAddr: loopbackListenAddr}
	server := mappingValue(root, "server")
	if server == nil {
		return settings, nil
	}
	if server.Kind != yaml.MappingNode {
		return desktopSettings{}, fmt.Errorf("invalid config %s: server must be a mapping", path)
	}

	if portNode := mappingValue(server, "port"); portNode != nil {
		if portNode.Kind != yaml.ScalarNode || portNode.Tag != "!!int" {
			return desktopSettings{}, fmt.Errorf("invalid server.port in %s: must be an integer", path)
		}
		if err := portNode.Decode(&settings.Port); err != nil {
			return desktopSettings{}, fmt.Errorf("invalid server.port in %s: %w", path, err)
		}
	}
	if err := validateServerPort(settings.Port); err != nil {
		return desktopSettings{}, fmt.Errorf("invalid server.port in %s: %w", path, err)
	}

	if addrNode := mappingValue(server, "listen_addr"); addrNode != nil {
		if addrNode.Kind != yaml.ScalarNode || addrNode.Tag != "!!str" {
			return desktopSettings{}, fmt.Errorf("invalid server.listen_addr in %s: must be a string", path)
		}
		if err := addrNode.Decode(&settings.ListenAddr); err != nil {
			return desktopSettings{}, fmt.Errorf("invalid server.listen_addr in %s: %w", path, err)
		}
	}
	if err := validateListenAddr(settings.ListenAddr); err != nil {
		return desktopSettings{}, fmt.Errorf("invalid server.listen_addr in %s: %w", path, err)
	}
	return settings, nil
}

// saveDesktopSettings writes the port and listen address into config.yaml,
// preserving the rest of the file.
func saveDesktopSettings(path string, settings desktopSettings) error {
	if err := validateServerPort(settings.Port); err != nil {
		return err
	}
	if err := validateListenAddr(settings.ListenAddr); err != nil {
		return err
	}
	if err := ensureDesktopConfig(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	document, root, err := parseDesktopConfig(path, data)
	if err != nil {
		return err
	}
	server, err := ensureMapping(root, "server", path)
	if err != nil {
		return err
	}
	setMappingValue(server, "port", integerNode(settings.Port))
	setMappingValue(server, "listen_addr", stringNode(settings.ListenAddr))
	return writeDesktopConfig(path, document)
}

func parseDesktopConfig(path string, data []byte) (*yaml.Node, *yaml.Node, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("invalid config %s: top level must be a mapping", path)
	}
	return &document, root, nil
}

func ensureMapping(parent *yaml.Node, key, path string) (*yaml.Node, error) {
	value := mappingValue(parent, key)
	if value == nil {
		value = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		appendMappingPair(parent, key, value)
		return value, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("invalid config %s: %s must be a mapping", path, key)
	}
	return value, nil
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	appendMappingPair(mapping, key, value)
}

func integerNode(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func writeDesktopConfig(path string, document *yaml.Node) error {
	data, err := yaml.Marshal(document)
	if err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
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
