package vhost

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/andrelcunha/ottermq/pkg/persistence"
	"github.com/rs/zerolog/log"
)

type Exchange struct {
	Name     string                `json:"name"`
	Typ      ExchangeType          `json:"type"`
	Bindings map[string][]*Binding `json:"bindings"`
	Props    *ExchangeProperties   `json:"properties"`
}

type ExchangeProperties struct {
	Passive    bool           `json:"passive"`
	Durable    bool           `json:"durable"`
	AutoDelete bool           `json:"auto_delete"`
	Internal   bool           `json:"internal"`
	NoWait     bool           `json:"no_wait"`
	Arguments  map[string]any `json:"arguments"`
}

type ExchangeType string

const (
	DIRECT ExchangeType = "direct"
	FANOUT ExchangeType = "fanout"
	TOPIC  ExchangeType = "topic"
)

type MandatoryExchange struct {
	Name string       `json:"name"`
	Type ExchangeType `json:"type"`
}

const (
	DEFAULT_EXCHANGE = "amq.default"
	EMPTY_EXCHANGE   = ""
	MANDATORY_TOPIC  = "amq.topic"
	MANDATORY_DIRECT = "amq.direct"
	MANDATORY_FANOUT = "amq.fanout"
)

var mandatoryExchanges = []MandatoryExchange{
	{Name: DEFAULT_EXCHANGE, Type: DIRECT},
	{Name: MANDATORY_TOPIC, Type: TOPIC},
	{Name: MANDATORY_DIRECT, Type: DIRECT},
	{Name: MANDATORY_FANOUT, Type: FANOUT},
}

// NewExchange creates a new Exchange instance with the given name, type, and properties.
func NewExchange(name string, typ ExchangeType, props *ExchangeProperties) *Exchange {
	if props == nil {
		props = &ExchangeProperties{
			Passive:    false,
			Durable:    false,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Arguments:  nil,
		}
	}
	return &Exchange{
		Name:     name,
		Typ:      typ,
		Bindings: make(map[string][]*Binding),
		Props:    props,
	}
}

// Candidate to be on an ExchangeManager interface
func ParseExchangeType(s string) (ExchangeType, error) {
	switch s {
	case string(DIRECT):
		return DIRECT, nil
	case string(FANOUT):
		return FANOUT, nil
	case string(TOPIC):
		return TOPIC, nil
	default:
		return "", fmt.Errorf("invalid exchange type: %s", s)
	}
}

func (vh *VHost) createMandatoryExchanges() {
	for _, mandatoryExchange := range mandatoryExchanges {
		if err := vh.CreateExchange(mandatoryExchange.Name, mandatoryExchange.Type, &ExchangeProperties{
			Durable:    false,
			AutoDelete: false,
			Internal:   false,
			NoWait:     false,
			Arguments:  nil,
		}); err != nil {
			log.Error().Err(err).Str("exchange", mandatoryExchange.Name).Msg("Failed to create mandatory exchange")
		}
	}
	vh.mu.Lock()
	defer vh.mu.Unlock()
	if defaultExchange, exists := vh.Exchanges[DEFAULT_EXCHANGE]; exists {
		vh.Exchanges[EMPTY_EXCHANGE] = defaultExchange
	}
}

// CreateExchange creates a new exchange with the given name, type, and properties and wires it into the vhost.
func (vh *VHost) CreateExchange(name string, typ ExchangeType, props *ExchangeProperties) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	// Check if the exchange already exists
	if existing, ok := vh.Exchanges[name]; ok {
		// return fmt.Errorf("exchange %s already exists", name)
		if props != nil && props.Passive {
			return nil
		}

		if existing.Typ != typ {
			return fmt.Errorf("exchange %s already exists with different type", name)
		}

		if existing.Props == nil || props == nil {
			return fmt.Errorf("exchange %s already exists with incompatible properties", name)
		}

		if existing.Props.Durable != props.Durable ||
			existing.Props.AutoDelete != props.AutoDelete ||
			existing.Props.Internal != props.Internal ||
			existing.Props.NoWait != props.NoWait ||
			!equalArgs(existing.Props.Arguments, props.Arguments) {
			return fmt.Errorf("exchange %s already exists with different properties", name)
		}

		log.Debug().Str("exchange", name).Msg("Exchange already exists with matching properties")
		return nil
	}
	if props != nil && props.Passive {
		return fmt.Errorf("exchange %s does not exist", name)
	}

	vh.Exchanges[name] = NewExchange(name, typ, props)
	// Handle durable property
	if props.Durable {
		if err := vh.persist.SaveExchangeMetadata(vh.Name, name, string(typ), props.ToPersistence()); err != nil {
			log.Error().Err(err).Str("exchange", name).Msg("Failed to save exchange metadata")
		}
	}
	return nil
}

func equalArgs(a, b map[string]any) bool {
	// Quick path
	if a == nil && b == nil {
		return true
	}
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ab) == string(bb)
}

// Internal helper: assumes vh.mu is already locked
func (vh *VHost) deleteExchangeUnlocked(name string) error {
	// If the exchange is the default exchange, return an error
	for _, mandatoryExchange := range mandatoryExchanges {
		if name == mandatoryExchange.Name {
			return fmt.Errorf("cannot delete default exchange")
		}
	}
	// Check if the exchange exists
	_, ok := vh.Exchanges[name]
	if !ok {
		return fmt.Errorf("exchange %s not found", name)
	}

	delete(vh.Exchanges, name)
	// Handle durable property
	if err := vh.persist.DeleteExchangeMetadata(vh.Name, name); err != nil {
		return fmt.Errorf("failed to delete exchange from persistence: %v", err)
	}
	log.Debug().Str("exchange", name).Msg("Deleted exchange")
	return nil
}

func (vh *VHost) DeleteExchange(name string) error {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	return vh.deleteExchangeUnlocked(name)
}

// checkAutoDeleteExchange checks if an exchange is auto-delete and has no bindings, and deletes it if so.
// It returns true if the exchange was deleted, false otherwise.
// Internal helper: assumes vh.mu is already locked
func (vh *VHost) checkAutoDeleteExchangeUnlocked(name string) (bool, error) {
	exchange, ok := vh.Exchanges[name]
	if !ok {
		return false, fmt.Errorf("exchange %s not found", name)
	}

	if exchange.Props.AutoDelete && len(exchange.Bindings) == 0 {
		log.Debug().Str("exchange", name).Msg("Auto-deleting exchange")
		if err := vh.deleteExchangeUnlocked(name); err != nil {
			return false, fmt.Errorf("failed to auto-delete exchange %s: %v", name, err)
		}
		return true, nil
	}
	return false, nil
}

// CheckAutoDeleteExchange checks if an exchange is auto-delete and has no bindings, and deletes it if so.
func (vh *VHost) CheckAutoDeleteExchange(name string) (bool, error) {
	vh.mu.Lock()
	defer vh.mu.Unlock()
	return vh.checkAutoDeleteExchangeUnlocked(name)
}

// ToPersistence convert ExchangeProperties to persistence format
func (ep *ExchangeProperties) ToPersistence() persistence.ExchangeProperties {
	return persistence.ExchangeProperties{
		// Passive:    ep.Passive, // Not needed in persistence
		Durable:    ep.Durable,
		AutoDelete: ep.AutoDelete,
		Internal:   ep.Internal,
		// NoWait:     ep.NoWait, // Not needed in persistence
		Arguments: ep.Arguments,
	}
}

// MatchTopic determines whether an AMQP topic exchange routing key matches a binding pattern.
//
// Parameters:
//
//	routingKey: The routing key of the published message (e.g., "foo.bar.baz").
//	patternKey: The binding pattern to match against (e.g., "foo.*.baz", "foo.#").
//
// Returns:
//
//	true if the routingKey matches the patternKey according to AMQP topic exchange semantics; false otherwise.
//
// AMQP Topic Matching Semantics:
//   - Words are dot-separated (e.g., "a.b.c").
//   - '*' matches exactly one word (e.g., "a.*.c" matches "a.b.c" but not "a.b.d.c").
//   - '#' matches zero or more words (e.g., "a.#" matches "a", "a.b", "a.b.c", etc.).
//   - A pattern of "#" matches any routing key.
//   - Empty words (e.g., "a..b", ".a", "a.") are considered invalid and do not match.
//   - Matching is case-sensitive.
//
// Reference: AMQP 0.9.1 topic exchange specification.
func MatchTopic(routingKey, patternKey string) bool {
	if patternKey == "#" {
		return true
	}
	if routingKey == patternKey {
		return true
	}

	routingWords := strings.Split(routingKey, ".")
	patternWords := strings.Split(patternKey, ".")

	if slices.Contains(routingWords, "") || slices.Contains(patternWords, "") {
		return false
	}
	// Special case: *.# requires at least two words
	if len(patternWords) == 2 && patternWords[0] == "*" && patternWords[1] == "#" {
		return len(routingWords) >= 2
	}
	return matchWords(routingWords, patternWords, 0, 0)
}

// matchWords recursively matches a routing key against a pattern key using AMQP topic wildcards.
//
// Parameters:
//
//	routing: slice of words from the routing key (e.g., "a.b.c" -> ["a", "b", "c"])
//	pattern: slice of words from the pattern key (e.g., "a.*.c" -> ["a", "*", "c"])
//	rIdx: current index in the routing slice
//	pIdx: current index in the pattern slice
//
// Algorithm:
//   - The function recursively advances through both slices, matching words according to the following rules:
//   - If both indices reach the end, it's a match.
//   - If the pattern is exhausted but routing remains, it's not a match.
//   - If routing is exhausted but pattern remains, only matches if all remaining pattern words are "#".
//   - For each pattern word:
//   - "#": matches zero or more routing words. Recursively try both:
//   - Advancing pattern index (zero words matched)
//   - Advancing routing index (one word matched)
//   - "*": matches exactly one routing word. Advance both indices.
//   - Literal: must match the current routing word. Advance both indices if matched.
func matchWords(routing, pattern []string, rIdx, pIdx int) bool {
	if pIdx == len(pattern) {
		return rIdx == len(routing)
	}

	if rIdx == len(routing) {
		// Only allow remaining # in pattern
		for pIdx < len(pattern) {
			if pattern[pIdx] != "#" {
				return false
			}
			pIdx++
		}
		return true
	}

	switch pattern[pIdx] {
	case "#":
		// Zero or more: try skipping #, or consuming one word
		return matchWords(routing, pattern, rIdx, pIdx+1) ||
			(rIdx < len(routing) && matchWords(routing, pattern, rIdx+1, pIdx))
	case "*":
		return matchWords(routing, pattern, rIdx+1, pIdx+1)
	default:
		if routing[rIdx] == pattern[pIdx] {
			return matchWords(routing, pattern, rIdx+1, pIdx+1)
		}
		return false
	}
}
