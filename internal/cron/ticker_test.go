package cron

import (
	"log"
	"testing"
	"time"
)

func TestCronTicker_Stop(t *testing.T) {
	ticker, _ := NewTicker("@daily")
	timeoutTimer := time.NewTimer(2 * time.Second)

	c := ticker.k
	ticker.Stop()
Outer:
	for {
		select {
		case <-c:
			break Outer
		case <-timeoutTimer.C:
			t.Fatal("Expected message on ticker 'k' channel within 2 seconds, but did not receive one")
		}
	}
}

func TestCronTicker_Reset_Error(t *testing.T) {
	ticker, _ := NewTicker("@daily")
	defer ticker.Stop()
	err := ticker.Reset("NOT_VALID_SCHEDULE", time.UTC)
	if err == nil {
		t.Fatal("should have gotten error, but received 'nil'")
	}
}

func TestCronTicker_Reset(t *testing.T) {
	ticker, _ := NewTicker("@daily")
	defer ticker.Stop()
	err := ticker.Reset("@monthly", time.UTC)
	if err != nil {
		t.Fatalf("expected 'nil', got: %q", err)
	}
}

func TestNewTicker_Error(t *testing.T) {
	_, err := NewTicker("NOT_VALID_SCHEDULE")
	if err == nil {
		t.Fatal("expected error, received 'nil'")
	}
}

func TestCronRunner_MultipleTicks(t *testing.T) {
	var counter int
	ticker, _ := NewTicker("*/1 * * * * ?")
	timeoutTimer := time.NewTimer(5 * time.Second)

Outer:
	for {
		select {
		case <-ticker.C:
			counter++
			if counter == 2 {
				break Outer
			}
		case <-timeoutTimer.C:
			t.Fatalf("timed out before second tick")
		}
	}
}

func ExampleNewTicker() {
	// The Cron schedule can be in Unix or Quartz format. Directives like
	// '@weekly' or '@daily' can also be parsed as defined in the
	// package github.com/robfig/cron/v3.

	// Example: "0 0 * * *"   -> Unix format: Daily at 12 AM UTC
	// Example: "0 0 0 * * ?" -> Quartz format: Daily at 12 AM UTC
	// Example: "@daily"      -> Directive: Every day at 12 AM UTC

	ticker, err := NewTicker("@daily")
	if err != nil {
		log.Fatal(err)
	}
	defer ticker.Stop()

	tick := <-ticker.C
	log.Print(tick)
}

// If you want to change the cron schedule of a ticker
// instead of creating a new one you can reset it.
func ExampleTicker_Reset() {
	ticker, err := NewTicker("0 0 0 ? * SUN")
	if err != nil {
		log.Fatal(err)
	}
	defer ticker.Stop()

	<-ticker.C
	log.Print("It's Sunday!")

	err = ticker.Reset("0 0 0 ? * WED", time.UTC)
	if err != nil {
		log.Fatal(err)
	}

	<-ticker.C
	log.Print("It's Wednesday!")
}

func ExampleTicker_Stop() {
	ticker, err := NewTicker("0 0 0 ? * SUN")
	if err != nil {
		log.Fatal(err)
	}
	defer ticker.Stop()

	<-ticker.C
}
