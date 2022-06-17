package ggthx

import (
	"log"
	"os"
)

type SyncTestBackend struct {
	callbacks     GGTHXSessionCallbacks
	sync          Sync
	numPlayers    int
	checkDistance int
	lastVerified  int
	rollingBack   bool
	running       bool
	logFp         os.File
	game          string

	currentInput GameInput
	lastInput    GameInput
	savedFrames  RingBuffer[savedInfo]
}

type savedInfo struct {
	frame    int
	checksum int
	buf      []byte
	cbuf     int
	input    GameInput
}

func (s *SyncTestBackend) DoPoll(timeout int) GGTHXErrorCode {
	if !s.running {
		var info GGTHXEvent
		info.code = GGTHX_EVENTCODE_RUNNING
		s.callbacks.OnEvent(&info)
		s.running = true
	}
	return GGTHX_OK
}

func (s *SyncTestBackend) AddPlayer(player *GGTHXPlayer, handle *GGTHXPlayerHandle) GGTHXErrorCode {
	if player.playerNum < 1 || player.playerNum > s.numPlayers {
		return GGTHX_ERRORCODE_PLAYER_OUT_OF_RANGE
	}
	*handle = (GGTHXPlayerHandle(player.playerNum - 1))
	return GGTHX_OK
}

func (s *SyncTestBackend) AddLocalInput(player GGTHXPlayerHandle, values []byte, size int) GGTHXErrorCode {
	if !s.running {
		return GGTHX_ERRORCODE_NOT_SYNCHRONIZED
	}

	index := int(player)
	for i := 0; i < size; i++ {
		s.currentInput.Bits[index*size+i] |= values[i]
	}
	return GGTHX_OK
}

func (s *SyncTestBackend) SyncInput(values []byte, size int, discconectFlags *int) GGTHXErrorCode {
	if s.rollingBack {
		s.lastInput = s.savedFrames.Front().input
	} else {
		if s.sync.GetFrameCount() == 0 {
			s.sync.SaveCurrentFrame()
		}
		s.lastInput = s.currentInput
	}
	copy(values, s.lastInput.Bits)
	if *discconectFlags > 0 {
		*discconectFlags = 0
	}
	return GGTHX_OK
}

func (s *SyncTestBackend) IncrementFrame() GGTHXErrorCode {
	s.sync.IncrementFrame()
	s.currentInput.Erase()

	if s.rollingBack {
		return GGTHX_OK
	}

	frame := s.sync.GetFrameCount()
	// Hold onto the current frame in our queue of saved states. We'll need
	// the checksum later to verify that our replay of the same frame got the
	// same results
	var info savedInfo
	info.frame = frame
	info.input = s.lastInput
	info.cbuf = s.sync.GetLastSavedFrame().cbuf
	copy(info.buf, s.sync.GetLastSavedFrame().buf)
	info.checksum = s.sync.GetLastSavedFrame().checksum
	s.savedFrames.Push(info)

	if frame-s.lastVerified == s.checkDistance {
		// We've gone far enough ahead and should now now start replaying frames
		// Load the last verified frame and set the rollback flag to true.
		s.sync.LoadFrame(s.lastVerified)

		s.rollingBack = true
		for !s.savedFrames.Empty() {
			s.callbacks.AdvanceFrame(0)

			// Verify that the checksum of this frame is the same as the one in our
			// list
			info = s.savedFrames.Front()
			s.savedFrames.Pop()

			if info.frame != s.sync.GetFrameCount() {
				log.Printf("Frame number %d does not match saved frame number %d", info.frame, frame)
				panic("RaiseSyncError")
			}
			checksum := s.sync.GetLastSavedFrame().checksum
			if info.checksum != checksum {
				// LogSaveStates(info)
				log.Printf("Checksum for frame %d does not match saved (%d != %d)", frame, checksum, info.checksum)
				panic("RaiseSyncError")
			}
			log.Printf("Checksum %08d for frame %d matches.\n", checksum, info.frame)
		}
		s.lastVerified = frame
		s.rollingBack = false
	}
	return GGTHX_OK
}

// We must 'impliment' these for this to be a true Session
func (s SyncTestBackend) Chat(text string) GGTHXErrorCode { return GGTHX_ERRORCODE_INVALID_REQUEST }
func (s SyncTestBackend) DisconnectPlayer(handle *GGTHXPlayerHandle) GGTHXErrorCode {
	return GGTHX_ERRORCODE_INVALID_REQUEST
}
func (s SyncTestBackend) GetNetworkStats(stats *GGTHXNetworkStats, handle GGTHXPlayerHandle) GGTHXErrorCode {
	return GGTHX_ERRORCODE_INVALID_REQUEST
}
func (s SyncTestBackend) Logv(format string, args ...int) GGTHXErrorCode {
	return GGTHX_ERRORCODE_INVALID_REQUEST
}
func (s SyncTestBackend) SetFrameDelay(player GGTHXPlayerHandle, delay int) GGTHXErrorCode {
	return GGTHX_ERRORCODE_INVALID_REQUEST
}
func (s SyncTestBackend) SetDisconnectTimeout(timeout int) GGTHXErrorCode {
	return GGTHX_ERRORCODE_INVALID_REQUEST
}
func (s SyncTestBackend) SetDisconnectNotifyStart(timeout int) GGTHXErrorCode {
	return GGTHX_ERRORCODE_INVALID_REQUEST
}
func (s SyncTestBackend) Close() GGTHXErrorCode { return GGTHX_ERRORCODE_INVALID_REQUEST }
