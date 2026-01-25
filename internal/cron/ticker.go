// Package cron creates a ticker (similar to time.Ticker) from a
// cron schedule.
//
// The Cron schedule can be in Unix or Quartz format. Directives like
// '@weekly' or '@daily' can also be parsed as defined in the
// package github.com/robfig/cron/v3.
//
// You may add the TimeZone/location to the beginning of the cron schedule
// to change the time zone. Default is UTC.
//
// See the NewTicker section for examples.
//
// Cronticker calculates the duration until the next scheduled 'tick'
// based on the cron schedule, and starts a new timer based on the
// duration calculated.
package cron

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

var scheduleParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Ticker is the struct returned to the user as a proxy
// to the ticker. The user can check the ticker channel for the next
// 'tick' via Ticker.C (similar to the user of time.Timer).
type Ticker struct {
	C chan time.Time
	k chan bool
}

// Stop sends the appropriate message on the control channel to
// kill the Ticker goroutines. It's good practice to use `defer Ticker.Stop()`.
func (c *Ticker) Stop() {
	c.k <- true
}

// Reset kills the ticker and starts it up again with
// the new schedule. The channel remains the same.
func (c *Ticker) Reset(schedule string, loc *time.Location) error {
	var err error
	c.Stop()
	err = newTicker(schedule, loc, c.C, c.k)
	if err != nil {
		return err
	}
	return nil
}

// NewTicker returns a Ticker for the UTC Time Zone.
func NewTicker(schedule string) (Ticker, error) {
	return NewTickerWithLocation(schedule, time.UTC)
}

// NewTickerWithLocation returns a Ticker based on a pre-defined time zone.
// You can check the ticker channel for the next tick by `Ticker.C`.
func NewTickerWithLocation(schedule string, loc *time.Location) (Ticker, error) {
	var cronTicker Ticker
	var err error

	cronTicker.C = make(chan time.Time, 1)
	cronTicker.k = make(chan bool, 1)

	err = newTicker(schedule, loc, cronTicker.C, cronTicker.k)
	if err != nil {
		return cronTicker, err
	}
	return cronTicker, nil
}

// newTicker prepares the channels, parses the schedule, and kicks off
// the goroutine that handles scheduling of each 'tick'.
func newTicker(schedule string, loc *time.Location, c chan time.Time, k <-chan bool) error {
	scheduleWithTZ := fmt.Sprintf("TZ=%s %s", loc.String(), schedule)
	cronSchedule, err := scheduleParser.Parse(scheduleWithTZ)
	if err != nil {
		return err
	}

	go cronRunner(cronSchedule, loc, c, k)

	return nil
}

// cronRunner handles calculating the next 'tick'. It communicates to
// the Ticker via a channel and will stop/return whenever it receives
// a bool on the `k` channel.
func cronRunner(schedule cron.Schedule, loc *time.Location, c chan time.Time, k <-chan bool) {
	nextTick := schedule.Next(time.Now().In(loc))
	timer := time.NewTimer(time.Until(nextTick))
	for {
		select {
		case <-k:
			timer.Stop()
			return
		case tickTime := <-timer.C:
			c <- tickTime
			nextTick = schedule.Next(tickTime.In(loc))
			timer.Reset(time.Until(nextTick))
		}
	}
}
