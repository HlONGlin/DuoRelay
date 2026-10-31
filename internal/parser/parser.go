package parser

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidFormat    = errors.New("invalid format")
	ErrUnsupportedProto = errors.New("unsupported protocol")
	ErrMissingField     = errors.New("missing required field")
	ErrInvalidField     = errors.New("invalid field value")
)

// Parser defines the interface for protocol parsers
type Parser interface {
	// Parse parses a share link or config into IR
	Parse(input string) (any, error)

	// CanParse checks if this parser can handle the input
	CanParse(input string) bool

	// Protocol returns the protocol name this parser handles
	Protocol() string
}

// Registry holds all protocol parsers
type Registry struct {
	parsers map[string]Parser
}

// NewRegistry creates a new parser registry
func NewRegistry() *Registry {
	return &Registry{
		parsers: make(map[string]Parser),
	}
}

// Register registers a parser
func (r *Registry) Register(p Parser) {
	r.parsers[p.Protocol()] = p
}

// Get returns a parser by protocol name
func (r *Registry) Get(protocol string) (Parser, error) {
	p, ok := r.parsers[protocol]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProto, protocol)
	}
	return p, nil
}

// ParseAny tries to parse with all registered parsers
func (r *Registry) ParseAny(input string) (any, error) {
	for _, p := range r.parsers {
		if p.CanParse(input) {
			return p.Parse(input)
		}
	}
	return nil, ErrUnsupportedProto
}

// GlobalRegistry is the default parser registry
var GlobalRegistry = NewRegistry()
