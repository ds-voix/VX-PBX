package report
// Shared access to Report structures

import (
	"errors"
	"sync"
	"time"
)

// Command execution result
type ExecResult struct {
    ID string             `json:"id"`
	Processed bool        `json:"processed"` // Was this command ever been processed?
	Command string        `json:"command"`
	Args []string         `json:"args"`
	Status int            `json:"status"`
	StdOut string         `json:"stdout"` // base64-encoded
	StdErr string         `json:"stderr"`
}

type Report struct {
	ClientID string       `json:"client_id"`
	MSG string            `json:"msg"` // message.Topic, message.Partition, message.Offset
	ERROR string          `json:"error,omitempty"`
	UUID string           `json:"uuid,omitempty"`
	TAG string            `json:"tag,omitempty"`
	PRODUCER string       `json:"producer,omitempty"`
	RESULTS []*ExecResult `json:"exec,omitempty"`
	TimeStamp string      `json:"timestamp,omitempty"`
}

type reports struct {
	ts time.Time
	rep []Report
}

type Reports struct {
	sync.RWMutex
	pool map[string]*reports
}

type Notifiers struct { // Notify on new report
	sync.RWMutex
	pool map[string](chan int)
}

type Timers struct { // Notify on timers: time_tick(s) and time_out
	sync.RWMutex
	pool map[string](chan int)
}


var (
	notifiers = Notifiers{ pool: make(map[string](chan int)) }
	timers = Timers{ pool: make(map[string](chan int)) }
)


func New() *Reports {
	return &Reports{ pool: make(map[string]*reports) }
}

func (r *Reports) add(report Report) (int) {
	r.Lock()
	defer r.Unlock()

if _, ok := r.pool[report.UUID]; !ok {
//	if r.pool[report.UUID] == nil { // New UUID
		var rep reports
		rep.ts = time.Now()
		rep.rep = []Report{report}
		// panic: assignment to entry in nil map ???
		r.pool[report.UUID] = &rep // make()
	} else { // Append to existing UUID
		r.pool[report.UUID].ts = time.Now()
		r.pool[report.UUID].rep = append(r.pool[report.UUID].rep, report)
	}
	return len(r.pool[report.UUID].rep) - 1 // Last index
}

func notify(uuid string, val int) {
	notifiers.RLock()
	defer notifiers.RUnlock()

	if notifiers.pool[uuid] != nil {
		notifiers.pool[uuid] <- val
	}
}

//	Store report, then try to notify caller party
func (r *Reports) Add(report Report) {
    if r == nil { return }
    if len(report.UUID) < 1 { return } // WTF?
	notify(report.UUID, r.add(report))
}

func (r *Reports) Get(uuid string, index int) (Report, error) {
	if r == nil {
		return Report{}, errors.New("Uninitialized *Reports")
	}
	r.RLock()
	defer r.RUnlock()

	if r.pool[uuid] != nil {
		if len(r.pool[uuid].rep) >= index + 1 {
			return r.pool[uuid].rep[index], nil
		} else {
			return Report{}, errors.New("Index out of range")
		}
	}
	return Report{}, errors.New("Empty pool[uuid]")
}

func (r *Reports) Del(uuid string) {
    if r == nil { return }
	r.Lock()
	defer r.Unlock()
	delete(r.pool, uuid)
}

func (r *Reports) Clean(age time.Duration) {
    if r == nil { return }
	r.Lock()
	defer r.Unlock()

	now := time.Now()
	for uuid, rep := range r.pool {
		expired := now.Sub(rep.ts)
		if expired > age {
			delete(r.pool, uuid)
		}
	}
}

// Send events about this uuid to buffered channel
// Also implement timer channel to simplify workflow
func AddNotifier(uuid string, length int, time_tick time.Duration, time_out time.Duration) (chan int, chan int) {
	notifiers.Lock()
	defer notifiers.Unlock()

	notifiers.pool[uuid] = make(chan int, length)

	if time_tick > 0 {
		timers.pool[uuid] = make(chan int)
		go func (time_tick time.Duration, time_out time.Duration) {
			var total time.Duration = 0
			count := 0
			for {
				time.Sleep(time_tick)
				total += time_tick
				count++
				timers.RLock() // Can't defer inside loop
				if _, ok := timers.pool[uuid]; !ok { // Already was destroyed
					timers.RUnlock()
					return
				}
				if total >= time_out {
					timers.pool[uuid] <- -1
					timers.RUnlock()
					return
				}
				timers.pool[uuid] <- count
				timers.RUnlock()
			}
		} (time_tick, time_out) // Avoid using args from outer func: less tangle for GC
		return notifiers.pool[uuid], timers.pool[uuid]
	}

	return notifiers.pool[uuid], nil
}

func DelNotifier(uuid string) {
	notifiers.Lock()
	defer notifiers.Unlock()
	if notifiers.pool[uuid] != nil {
		close(notifiers.pool[uuid])
		delete(notifiers.pool, uuid)
	}

	timers.Lock()
	defer timers.Unlock()
	if timers.pool[uuid] != nil {
		close(timers.pool[uuid])
		delete(timers.pool, uuid)
	}
}
