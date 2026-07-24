package main

import (
	"errors"
	"testing"
)

func newTestApp() (*CommandBus, *QueryBus, *WriteStore) {
	bus := NewBus()
	writeStore := NewWriteStore()
	readStore := NewReadStore()
	Project(bus, readStore)
	return NewCommandBus(writeStore, bus), NewQueryBus(readStore), writeStore
}

func TestReadModel_HasADifferentShapeThanTheWriteModel(t *testing.T) {
	cmds, queries, writeStore := newTestApp()

	_ = cmds.PlaceOrder("ord-1")
	_ = cmds.AddItem("ord-1", "keyboard", 120)
	_ = cmds.AddItem("ord-1", "mouse", 40)

	// The write side keeps every line, because rules are checked against them.
	order, _ := writeStore.Get("ord-1")
	if len(order.Lines) != 2 {
		t.Fatalf("write model should keep 2 lines, got %d", len(order.Lines))
	}

	// The read side keeps no lines at all - only the totals a screen renders.
	summary, _ := queries.GetSummary("ord-1")
	if summary.ItemCount != 2 || summary.Total != 160 {
		t.Fatalf("expected items=2 total=160, got items=%d total=%v", summary.ItemCount, summary.Total)
	}
}

func TestReadModel_OnlyLearnsAboutChangesThroughEvents(t *testing.T) {
	cmds, queries, writeStore := newTestApp()

	_ = cmds.PlaceOrder("ord-1")
	_ = cmds.AddItem("ord-1", "keyboard", 120)

	// Write directly to the store, bypassing the command bus - so no event.
	order, _ := writeStore.Get("ord-1")
	order.Lines = append(order.Lines, OrderLine{Item: "smuggled", Price: 500})
	writeStore.Save(order)

	summary, _ := queries.GetSummary("ord-1")
	// Shared storage would report 620 here. It reports 120, which proves the
	// event is the only thing connecting the two sides.
	if summary.Total != 120 {
		t.Fatalf("read model must not see an unpublished write, got total=%v", summary.Total)
	}
}

func TestCommands_AreValidatedAgainstTheWriteModel(t *testing.T) {
	cmds, _, _ := newTestApp()

	if err := cmds.AddItem("nope", "x", 1); !errors.Is(err, ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}

	_ = cmds.PlaceOrder("ord-1")
	if err := cmds.PayOrder("ord-1"); !errors.Is(err, ErrEmptyOrder) {
		t.Fatalf("expected ErrEmptyOrder, got %v", err)
	}

	_ = cmds.AddItem("ord-1", "keyboard", 120)
	_ = cmds.PayOrder("ord-1")
	if err := cmds.AddItem("ord-1", "late", 1); !errors.Is(err, ErrOrderNotOpen) {
		t.Fatalf("expected ErrOrderNotOpen, got %v", err)
	}
}

func TestRejectedCommand_LeavesTheReadModelUntouched(t *testing.T) {
	cmds, queries, _ := newTestApp()

	_ = cmds.PlaceOrder("ord-1")
	_ = cmds.AddItem("ord-1", "keyboard", 120)
	_ = cmds.PayOrder("ord-1")

	before, _ := queries.GetSummary("ord-1")
	_ = cmds.AddItem("ord-1", "late", 999) // rejected: the order is paid
	after, _ := queries.GetSummary("ord-1")

	// A rejected command publishes no event, so the projection never runs.
	if after != before {
		t.Fatalf("rejected command moved the read model: %+v -> %+v", before, after)
	}
}
