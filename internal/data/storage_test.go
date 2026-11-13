package data

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"testing"
	"time"
)

type testConnector struct{}

func (testConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, fmt.Errorf("No SQL is used in this pool-isolation test.")
}
func (testConnector) Driver() driver.Driver { return testDriver{} }

type testDriver struct{}

func (testDriver) Open(string) (driver.Conn, error) { return nil, fmt.Errorf("Not used") }
func TestSlowOpenDoesNotBlockAnotherResource(t *testing.T) {
	var p Pools
	defer p.Close()
	open := func() (*sql.DB, error) { return sql.OpenDB(testConnector{}), nil }
	_, release, e := p.Acquire(context.Background(), "healthy", open)
	if e != nil {
		t.Fatal(e)
	}
	release()
	started, finish, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		p.Acquire(context.Background(), "slow", func() (*sql.DB, error) { close(started); <-finish; return nil, fmt.Errorf("Simulated failure") })
	}()
	<-started
	ready := make(chan error, 1)
	go func() {
		_, release, e := p.Acquire(context.Background(), "healthy", open)
		if e == nil {
			release()
		}
		ready <- e
	}()
	select {
	case e := <-ready:
		if e != nil {
			t.Error(e)
		}
	case <-time.After(time.Second):
		t.Error("A slow database blocked a healthy cached resource")
	}
	close(finish)
	<-done
}
