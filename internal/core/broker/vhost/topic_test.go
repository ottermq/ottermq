package vhost

import "testing"

func TestMatchTopic(t *testing.T) {
	tests := []struct {
		name        string
		routingKey  string
		bindingKey  string
		shouldMatch bool
	}{
		// Exact matches
		{"exact match simple", "stock.usd", "stock.usd", true},
		{"exact match complex", "stock.usd.nasdaq.apple", "stock.usd.nasdaq.apple", true},
		{"no match different", "stock.usd", "stock.eur", false},
		{"no match prefix", "stock.usd", "stock", false},
		{"no match suffix", "stock", "stock.usd", false},

		// Single wildcard * (matches exactly one word)
		{"* matches one word end", "stock.usd", "stock.*", true},
		{"* matches one word start", "usd.stock", "*.stock", true},
		{"* matches one word middle", "stock.usd.nasdaq", "stock.*.nasdaq", true},
		{"* doesn't match zero words", "stock", "stock.*", false},
		{"* doesn't match two words", "stock.usd.nasdaq", "stock.*", false},
		{"* wrong word", "stock.usd.nasdaq", "stock.*.nyse", false},
		{"multiple * match", "a.b.c", "*.*.c", true},
		{"multiple * no match", "a.b", "*.*.c", false},
		{"all * match", "a.b.c", "*.*.*", true},
		{"all * wrong count", "a.b", "*.*.*", false},

		// Hash wildcard # (matches zero or more words)
		{"# matches zero words", "stock", "stock.#", true},
		{"# matches one word", "stock.usd", "stock.#", true},
		{"# matches two words", "stock.usd.nasdaq", "stock.#", true},
		{"# matches many words", "a.b.c.d.e.f", "a.#", true},
		{"# at start zero", "stock", "#.stock", true},
		{"# at start one", "usd.stock", "#.stock", true},
		{"# at start many", "a.b.c.stock", "#.stock", true},
		{"# in middle", "a.b.c", "a.#.c", true},
		{"# in middle zero", "a.c", "a.#.c", true},
		{"# catch-all", "anything.at.all", "#", true},
		{"# catch-all single", "word", "#", true},
		{"# no match different end", "stock.usd", "#.eur", false},

		// Complex patterns with # and *
		{"# and * match", "a.b.c.d", "#.*.d", true},
		{"# and * match end", "a.b.c", "a.#.*", true},
		{"# and * no match", "a.b", "#.*.d", false},
		{"*.# pattern", "a.b.c", "*.#", true},
		{"*.# single word", "a", "*.#", false},
		{"#.* pattern", "a.b.c", "#.*", true},
		{"#.*.# complex", "a.b.c.d.e", "#.*.#", true},

		// Edge cases
		{"empty routing key with #", "", "#", true},
		{"empty routing key with *", "", "*", false},
		{"empty routing key with literal", "", "word", false},
		{"both empty", "", "", true},
		{"routing empty binding not", "word", "", false},

		// Real-world logging examples
		{"error logs exact level", "app.database.error", "*.*.error", true},
		{"error logs any component", "web.error", "#.error", true},
		{"error logs no match", "app.database.info", "*.*.error", false},
		{"all database logs", "api.database.info", "*.database.#", true},
		{"all database logs short", "web.database", "*.database.#", true},
		{"specific database error", "api.database.error", "*.database.error", true},

		// Real-world stock examples
		{"stock USD any exchange", "stock.usd.nasdaq", "stock.usd.*", true},
		{"stock any currency NASDAQ", "stock.eur.nasdaq", "stock.*.nasdaq", true},
		{"all stocks", "stock.usd.nyse", "stock.#", true},
		{"stock no match currency", "stock.usd.nasdaq", "stock.eur.*", false},

		// Real-world event examples
		{"tenant events", "tenant1.order.created", "tenant1.#", true},
		{"all orders", "tenant2.order.created", "*.order.*", true},
		{"specific event", "tenant1.order.created", "tenant1.order.created", true},
		{"user events", "tenant1.user.login", "*.user.*", true},

		// Tricky cases
		{"# at end with extra", "a.b", "a.b.#", true},
		{"# multiple in pattern", "a.b.c", "#.#", true},
		{"# between words", "a.x.y.b", "a.#.b", true},
		{"* matches literal # in routing", "a.#.b", "a.*.b", true},   // # is literal in routing key
		{"literal # in routing exact match", "a.#.b", "a.#.b", true}, // Exact match
		{"literal * in routing exact match", "a.*.b", "a.*.b", true}, // * is literal in routing key

		// Consecutive wildcards
		{"**.pattern", "a.b.c", "*.*", false},
		{"##.pattern", "a.b.c", "#.#", true},
		{"*#.pattern", "a.b.c", "*.#", true},
		{"#*.pattern", "a.b", "#.*", true},

		// Long patterns
		{"long exact", "a.b.c.d.e.f.g", "a.b.c.d.e.f.g", true},
		{"long with #", "a.b.c.d.e.f.g", "a.#.g", true},
		{"long with *", "a.b.c.d.e.f.g", "a.*.c.*.e.*.g", true},
		{"long mixed", "a.b.c.d.e.f.g.h", "a.#.d.*.f.#", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchTopic(tt.routingKey, tt.bindingKey)
			if result != tt.shouldMatch {
				t.Errorf("MatchTopic(%q, %q) = %v, want %v",
					tt.routingKey, tt.bindingKey, result, tt.shouldMatch)
			}
		})
	}
}

// TestMatchTopicSymmetry ensures that the function behaves consistently
func TestMatchTopicSymmetry(t *testing.T) {
	// These should all be true
	symmetricTrue := []struct {
		routing string
		binding string
	}{
		{"a.b.c", "a.b.c"},
		{"test", "#"},
		{"", ""},
	}

	for _, tt := range symmetricTrue {
		if !MatchTopic(tt.routing, tt.binding) {
			t.Errorf("Expected MatchTopic(%q, %q) to be true", tt.routing, tt.binding)
		}
	}

	// Test that certain patterns are NOT symmetric
	if MatchTopic("a.b", "a.*.*") {
		t.Error("MatchTopic should not match when pattern has more words than routing")
	}
}

// TestMatchTopicSpecialCases tests edge cases that might cause issues
func TestMatchTopicSpecialCases(t *testing.T) {
	tests := []struct {
		name        string
		routingKey  string
		bindingKey  string
		shouldMatch bool
	}{
		// Dots at boundaries (invalid in real AMQP but test robustness)
		{"no leading dots", ".a.b", "*.b", false},  // Malformed
		{"no trailing dots", "a.b.", "a.*", false}, // Malformed
		{"no double dots", "a..b", "a.*.b", false}, // Malformed

		// Only wildcards
		{"only #", "", "#", true},
		{"only *", "word", "*", true},
		{"only * empty", "", "*", false},
		{"double #", "a.b", "#.#", true},
		{"triple *", "a.b.c", "*.*.*", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchTopic(tt.routingKey, tt.bindingKey)
			if result != tt.shouldMatch {
				t.Errorf("MatchTopic(%q, %q) = %v, want %v",
					tt.routingKey, tt.bindingKey, result, tt.shouldMatch)
			}
		})
	}
}
