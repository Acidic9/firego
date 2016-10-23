package firego

import (
	"strings"
	syncc "sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zabawaba99/firego/internal/firetest"
	"github.com/zabawaba99/firego/sync"
)

type testEvent struct {
	snapshot    DataSnapshot
	previousKey string
}

type testEvents struct {
	mtx    syncc.Mutex
	events []testEvent
}

func (te *testEvents) add(event testEvent) {
	te.mtx.Lock()
	te.events = append(te.events, event)
	te.mtx.Unlock()
}

func (te *testEvents) get(i int) testEvent {
	te.mtx.Lock()
	event := te.events[i]
	te.mtx.Unlock()
	return event
}

func (te *testEvents) len() int {
	te.mtx.Lock()
	l := len(te.events)
	te.mtx.Unlock()
	return l
}

func TestChildAdded(t *testing.T) {
	server := firetest.New()
	server.Start()
	defer server.Close()

	fb := New(server.URL, nil)

	// set some existing values that should come down
	server.Set("something", true)
	server.Set("AAA", "foo")

	// use this to sync up between different events
	allNotifications, addNotifications := make(chan Event), make(chan Event)
	err := fb.Watch(allNotifications)
	require.NoError(t, err)
	readNotification(t, allNotifications)

	var results []testEvent
	var mtx syncc.Mutex
	fn := func(snapshot DataSnapshot, previousChildKey string) {
		mtx.Lock()
		results = append(results, testEvent{snapshot, previousChildKey})
		addNotifications <- Event{Path: snapshot.Key}
		mtx.Unlock()
	}
	err = fb.ChildAdded(fn)
	require.NoError(t, err)

	// read the two events that are already there
	readNotification(t, addNotifications)
	readNotification(t, addNotifications)

	// should get regular addition events
	err = fb.Child("foo").Set(2)
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, addNotifications)

	err = fb.Set(map[string]string{"lala": "faa", "alal": "aaf"})
	require.NoError(t, err)
	readNotification(t, allNotifications)
	// read the two events we should have gotten
	readNotification(t, addNotifications)
	readNotification(t, addNotifications)

	err = fb.Child("bar").Set(map[string]string{"hi": "mom"})
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, addNotifications)

	fbChild, err := fb.Push("gaga oh la la")
	require.NoError(t, err)
	pushKey := strings.TrimPrefix(fbChild.url, fb.url+"/")
	readNotification(t, allNotifications)
	readNotification(t, addNotifications)

	// should not get updates
	err = fb.Child("foo").Set(false)
	require.NoError(t, err)
	readNotification(t, allNotifications)

	// or deletes
	err = fb.Child("bar").Remove()
	require.NoError(t, err)
	readNotification(t, allNotifications)

	// should get a notification after adding a deleted field
	err = fb.Child("bar").Set("something-else")
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, addNotifications)

	// should not get notifications for addition to a child not
	err = fb.Child("bar/child").Set(true)
	require.NoError(t, err)
	readNotification(t, allNotifications)

	expected := []testEvent{
		{newSnapshot(sync.NewNode("AAA", "foo")), ""},
		{newSnapshot(sync.NewNode("something", true)), "AAA"},
		{newSnapshot(sync.NewNode("foo", float64(2))), "something"},
		{newSnapshot(sync.NewNode("alal", "aaf")), "foo"},
		{newSnapshot(sync.NewNode("lala", "faa")), "alal"},
		{newSnapshot(sync.NewNode("bar", map[string]interface{}{"hi": "mom"})), "lala"},
		{newSnapshot(sync.NewNode(pushKey, "gaga oh la la")), "bar"},
		{newSnapshot(sync.NewNode("bar", "something-else")), pushKey},
	}

	mtx.Lock()
	defer mtx.Unlock()

	require.Len(t, results, len(expected))
	for i, v := range expected {
		r := results[i]

		assert.EqualValues(t, v.previousKey, r.previousKey, "PK do not match, index %d", i)
		assert.EqualValues(t, v.snapshot, r.snapshot, "Snapshots do not match, index %d", i)
	}
}

func TestChildChanged(t *testing.T) {
	server := firetest.New()
	server.Start()
	defer server.Close()

	fb := New(server.URL, nil)

	// set some existing values that should come down
	server.Set("something", true)
	server.Set("AAA", "foo")
	server.Set("foo", 432)
	server.Set("lala", "123")
	server.Set("alal", "3333")
	server.Set("bar", 12123123123)

	// use this to sync up between different events
	allNotifications, changedNotifications := make(chan Event), make(chan Event)
	err := fb.Watch(allNotifications)
	require.NoError(t, err)
	readNotification(t, allNotifications)

	var results []testEvent
	var mtx syncc.Mutex
	fn := func(snapshot DataSnapshot, previousChildKey string) {
		mtx.Lock()
		results = append(results, testEvent{snapshot, previousChildKey})
		changedNotifications <- Event{Path: snapshot.Key}
		mtx.Unlock()
	}
	err = fb.ChildChanged(fn)
	require.NoError(t, err)

	// should get regular update events
	err = fb.Child("foo").Set(2)
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, changedNotifications)

	err = fb.Set(map[string]string{"lala": "faa", "alal": "aaf"})
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, changedNotifications)
	readNotification(t, changedNotifications)

	err = fb.Child("bar").Set(map[string]string{"hi": "mom"})
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, changedNotifications)

	// should not get push
	_, err = fb.Push("gaga oh la la")
	require.NoError(t, err)
	readNotification(t, allNotifications)

	// should not get adds
	err = fb.Child("foo123123").Set(false)
	require.NoError(t, err)
	readNotification(t, allNotifications)

	// or deletes
	err = fb.Child("something").Remove()
	require.NoError(t, err)
	readNotification(t, allNotifications)

	// should not get notifications for addition to a child
	err = fb.Child("bar/child").Set(true)
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, changedNotifications)

	expected := []testEvent{
		{newSnapshot(sync.NewNode("foo", float64(2))), ""},
		{newSnapshot(sync.NewNode("alal", "aaf")), "foo"},
		{newSnapshot(sync.NewNode("lala", "faa")), "alal"},
		{newSnapshot(sync.NewNode("bar", map[string]interface{}{"hi": "mom"})), "lala"},
		{newSnapshot(sync.NewNode("bar", map[string]interface{}{"hi": "mom", "child": true})), "bar"},
	}

	mtx.Lock()
	defer mtx.Unlock()

	require.Len(t, results, len(expected))
	for i, v := range expected {
		r := results[i]

		assert.EqualValues(t, v.previousKey, r.previousKey, "PK do not match, index %d", i)
		assert.EqualValues(t, v.snapshot, r.snapshot, "Snapshots do not match, index %d", i)
	}
}

func TestChildRemoved(t *testing.T) {
	server := firetest.New()
	server.Start()
	defer server.Close()

	fb := New(server.URL, nil).Child("foo")

	// set some existing values that should come down
	server.Set("foo/something", true)
	server.Set("foo/AAA", "foo")

	// use this to sync up between different events
	allNotifications, removedNotifications := make(chan Event), make(chan Event)
	err := fb.Watch(allNotifications)
	require.NoError(t, err)
	readNotification(t, allNotifications)

	var results []testEvent
	var mtx syncc.Mutex
	fn := func(snapshot DataSnapshot, previousChildKey string) {
		mtx.Lock()
		results = append(results, testEvent{snapshot, previousChildKey})
		removedNotifications <- Event{Path: snapshot.Key}
		mtx.Unlock()
	}
	err = fb.ChildRemoved(fn)
	require.NoError(t, err)

	// should get regular deletion events
	err = fb.Child("AAA").Remove()
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, removedNotifications)

	err = fb.Child("something").Remove()
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, removedNotifications)

	// should get event for something that was deleted that
	// was created after connection was established
	err = fb.Child("foobar").Set("eep!")
	require.NoError(t, err)
	readNotification(t, allNotifications)

	err = fb.Child("foobar").Remove()
	require.NoError(t, err)
	readNotification(t, allNotifications)
	readNotification(t, removedNotifications)

	err = fb.Child("troll1").Set("yes1")
	require.NoError(t, err)
	readNotification(t, allNotifications)
	err = fb.Child("troll2").Set("yes2")
	require.NoError(t, err)
	readNotification(t, allNotifications)
	err = fb.Child("troll3").Set("yes3")
	require.NoError(t, err)
	readNotification(t, allNotifications)

	err = fb.Remove()
	require.NoError(t, err)
	// we should get 3 events since we're removing 3 keys
	readNotification(t, allNotifications)
	readNotification(t, removedNotifications)
	readNotification(t, removedNotifications)
	readNotification(t, removedNotifications)

	expected := []testEvent{
		{newSnapshot(sync.NewNode("AAA", "foo")), ""},
		{newSnapshot(sync.NewNode("something", true)), ""},
		{newSnapshot(sync.NewNode("foobar", "eep!")), ""},
		{newSnapshot(sync.NewNode("troll1", "yes1")), ""},
		{newSnapshot(sync.NewNode("troll2", "yes2")), ""},
		{newSnapshot(sync.NewNode("troll3", "yes3")), ""},
	}

	mtx.Lock()
	defer mtx.Unlock()

	require.Len(t, results, len(expected))
	for i, v := range expected {
		r := results[i]

		assert.EqualValues(t, v.previousKey, r.previousKey, "PK do not match, index %d", i)
		assert.EqualValues(t, v.snapshot, r.snapshot, "Snapshots do not match, index %d", i)
	}
}

func TestRemoveEventFunc(t *testing.T) {
	server := firetest.New()
	server.Start()
	defer server.Close()

	fb := New(server.URL, nil)
	// use this to sync up between different events
	allNotifications := make(chan Event)
	err := fb.Watch(allNotifications)
	require.NoError(t, err)
	readNotification(t, allNotifications)

	fn := func(snapshot DataSnapshot, previousChildKey string) {
		assert.Fail(t, "Should not have received anything")
	}
	err = fb.ChildAdded(fn)
	require.NoError(t, err)

	fb.RemoveEventFunc(fn)

	fb.Child("hello").Set(false)
	readNotification(t, allNotifications)

	assert.Len(t, fb.eventFuncs, 0)
}

func readNotification(t *testing.T, notification chan Event) {
	select {
	case <-notification:
	case <-time.After(1 * time.Second):
		require.FailNow(t, "timed out reading notification")
	}
}
