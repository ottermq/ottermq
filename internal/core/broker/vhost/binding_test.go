package vhost

import (
	"testing"

	"github.com/andrelcunha/ottermq/pkg/persistence/implementations/dummy"
)

func TestDeleteBinding_AutoDeleteExchange(t *testing.T) {
	vh := &VHost{
		Exchanges: make(map[string]*Exchange),
		Queues:    make(map[string]*Queue),
		persist:   &dummy.DummyPersistence{},
	}
	// Create exchange with auto-delete
	ex := &Exchange{
		Name:     "ex1",
		Typ:      DIRECT,
		Bindings: make(map[string][]*Queue),
		Props:    &ExchangeProperties{AutoDelete: true},
	}
	vh.Exchanges["ex1"] = ex
	// Create queue and bind
	q := &Queue{Name: "q1"}
	ex.Bindings["rk"] = []*Queue{q}

	// Delete binding
	err := vh.DeleteBindingUnlocked(ex, "q1", "rk")
	if err != nil {
		t.Fatalf("DeleteBinding failed: %v", err)
	}
	// Exchange should be auto-deleted
	if _, exists := vh.Exchanges["ex1"]; exists {
		t.Errorf("Expected exchange to be auto-deleted, but it still exists")
	}
}

func TestDeleteBinding_NoAutoDeleteExchange(t *testing.T) {
	vh := &VHost{
		Exchanges: make(map[string]*Exchange),
		Queues:    make(map[string]*Queue),
		persist:   &dummy.DummyPersistence{},
	}
	ex := &Exchange{
		Name:     "ex2",
		Typ:      DIRECT,
		Bindings: make(map[string][]*Queue),
		Props:    &ExchangeProperties{AutoDelete: false},
	}
	vh.Exchanges["ex2"] = ex
	q := &Queue{Name: "q2"}
	ex.Bindings["rk"] = []*Queue{q}

	err := vh.DeleteBindingUnlocked(ex, "q2", "rk")
	if err != nil {
		t.Fatalf("DeleteBinding failed: %v", err)
	}
	// Exchange should NOT be auto-deleted
	if _, exists := vh.Exchanges["ex2"]; !exists {
		t.Errorf("Expected exchange to remain, but it was deleted")
	}
}

func TestDeleteBinding_QueueNotFound(t *testing.T) {
	vh := &VHost{
		Exchanges: make(map[string]*Exchange),
		Queues:    make(map[string]*Queue),
		persist:   &dummy.DummyPersistence{},
	}
	ex := &Exchange{
		Name:     "ex3",
		Typ:      DIRECT,
		Bindings: make(map[string][]*Queue),
		Props:    &ExchangeProperties{AutoDelete: true},
	}
	vh.Exchanges["ex3"] = ex
	q := &Queue{Name: "q3"}
	ex.Bindings["rk"] = []*Queue{q}

	err := vh.DeleteBindingUnlocked(ex, "notfound", "rk")
	if err == nil {
		t.Errorf("Expected error for queue not found, got nil")
	}
}

func TestDeleteBinding_BindingNotFound(t *testing.T) {
	vh := &VHost{
		Exchanges: make(map[string]*Exchange),
		Queues:    make(map[string]*Queue),
		persist:   &dummy.DummyPersistence{},
	}
	ex := &Exchange{
		Name:     "ex4",
		Typ:      DIRECT,
		Bindings: make(map[string][]*Queue),
		Props:    &ExchangeProperties{AutoDelete: true},
	}
	vh.Exchanges["ex4"] = ex

	err := vh.DeleteBindingUnlocked(ex, "q4", "notfound")
	if err == nil {
		t.Errorf("Expected error for binding not found, got nil")
	}
}
