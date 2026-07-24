package main

import (
	"errors"
	"reflect"
	"testing"
)

func newTestSaga() (*Saga, *InventoryService, *PaymentService, *ShippingService) {
	inv := NewInventoryService(map[string]bool{"no-stock": true})
	pay := NewPaymentService(
		map[string]bool{"declined": true},
		map[string]bool{"refund-broken": true},
	)
	ship := NewShippingService(map[string]bool{
		"no-courier":    true,
		"refund-broken": true,
	})
	return buildSaga(inv, pay, ship, nil), inv, pay, ship
}

func TestSaga_AllStepsSucceedLeavesEverythingCommitted(t *testing.T) {
	saga, inv, pay, ship := newTestSaga()

	res := saga.Execute("happy")

	if !res.OK() {
		t.Fatalf("expected success, failed at %s: %v", res.FailedStep, res.FailureCause)
	}
	want := []string{"ReserveStock", "ChargePayment", "CreateShipment"}
	if !reflect.DeepEqual(res.Completed, want) {
		t.Fatalf("expected steps %v, got %v", want, res.Completed)
	}
	if len(res.Compensated) != 0 {
		t.Fatalf("nothing should be compensated on the happy path, got %v", res.Compensated)
	}
	if !inv.IsReserved("happy") || !pay.IsCharged("happy") || !ship.HasShipment("happy") {
		t.Fatal("all three services should hold committed state")
	}
}

func TestSaga_FailureOnFirstStepCompensatesNothing(t *testing.T) {
	saga, inv, pay, _ := newTestSaga()

	res := saga.Execute("no-stock")

	if res.FailedStep != "ReserveStock" {
		t.Fatalf("expected failure at ReserveStock, got %q", res.FailedStep)
	}
	if !errors.Is(res.FailureCause, ErrOutOfStock) {
		t.Fatalf("expected ErrOutOfStock, got %v", res.FailureCause)
	}
	// Nothing succeeded, so there is nothing to undo. A saga that "rolls back"
	// here would be undoing work that never happened.
	if len(res.Compensated) != 0 {
		t.Fatalf("expected no compensation, got %v", res.Compensated)
	}
	if inv.IsReserved("no-stock") || pay.IsCharged("no-stock") {
		t.Fatal("no service should hold state after a first-step failure")
	}
}

func TestSaga_FailureInTheMiddleCompensatesOnlyCompletedSteps(t *testing.T) {
	saga, inv, pay, _ := newTestSaga()

	res := saga.Execute("declined")

	if res.FailedStep != "ChargePayment" {
		t.Fatalf("expected failure at ChargePayment, got %q", res.FailedStep)
	}
	want := []string{"ReserveStock"}
	if !reflect.DeepEqual(res.Compensated, want) {
		t.Fatalf("expected %v compensated, got %v", want, res.Compensated)
	}
	if inv.IsReserved("declined") {
		t.Fatal("stock must be released after the payment step failed")
	}
	if pay.IsCharged("declined") {
		t.Fatal("a declined charge must not leave the order marked as paid")
	}
}

func TestSaga_CompensationRunsInReverseOrder(t *testing.T) {
	saga, inv, pay, ship := newTestSaga()

	res := saga.Execute("no-courier")

	if res.FailedStep != "CreateShipment" {
		t.Fatalf("expected failure at CreateShipment, got %q", res.FailedStep)
	}
	// Steps ran Reserve -> Charge, so undoing must go Charge -> Reserve.
	// Reverse order matters because a later step can depend on an earlier one.
	want := []string{"ChargePayment", "ReserveStock"}
	if !reflect.DeepEqual(res.Compensated, want) {
		t.Fatalf("expected compensation order %v, got %v", want, res.Compensated)
	}
	if inv.IsReserved("no-courier") || pay.IsCharged("no-courier") || ship.HasShipment("no-courier") {
		t.Fatal("every service should be back to its pre-saga state")
	}
}

func TestSaga_FailedCompensationLeavesTheSystemInconsistent(t *testing.T) {
	saga, inv, pay, _ := newTestSaga()

	res := saga.Execute("refund-broken")

	// The refund failed, so ChargePayment never makes it into Compensated -
	// but the saga keeps going and still releases the stock.
	want := []string{"ReserveStock"}
	if !reflect.DeepEqual(res.Compensated, want) {
		t.Fatalf("expected only %v compensated, got %v", want, res.Compensated)
	}
	if inv.IsReserved("refund-broken") {
		t.Fatal("stock should still have been released")
	}
	// This is the honest, uncomfortable outcome: the customer paid for an
	// order that will never ship. No retry loop fixes it - a human must.
	if !pay.IsCharged("refund-broken") {
		t.Fatal("the charge should still stand, since the refund failed")
	}
}
