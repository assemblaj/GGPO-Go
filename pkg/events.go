package ggpo

type EventCode int

const (
	EventCodeConnectedToPeer       EventCode = 1000
	EventCodeSynchronizingWithPeer EventCode = 1001
	EventCodeSynchronizedWithPeer  EventCode = 1002
	EventCodeRunning               EventCode = 1003
	EventCodeDisconnectedFromPeer  EventCode = 1004
	EventCodeTimeSync              EventCode = 1005
	EventCodeConnectionInterrupted EventCode = 1006
	EventCodeConnectionResumed     EventCode = 1007
)

// the original had a union a named struct for each event type,
// but mostly each just had the PlayerHandle, so instead i'm
// including the extra specific fields in every event object
// Hopefully I remember I did this like this
type Event struct {
	Code              EventCode
	player            PlayerHandle
	count             int // synchronizing
	total             int // synchronizing
	framesAhead       int // timesync
	disconnectTimeout int // connection interrupted
}
