package ggpo_test

import (
	"bytes"
	"testing"

	"github.com/assemblaj/ggpo/internal/mocks"

	"github.com/assemblaj/ggpo"
)

func TestNewSyncTestBackend(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 1)
	stb := ggpo.NewSyncTest(&session, 1, 8, 4, true)
	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)

}

func TestSyncTestBackendAddPlayerOver(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 2)
	stb := ggpo.NewSyncTest(&session, 1, 8, 4, true)
	var handle ggpo.PlayerHandle
	err := stb.AddPlayer(&player, &handle)
	if err == nil {
		t.Errorf("There should be an error for adding a player greater than the total num Players.")
	}
}

func TestSyncTestBackendAddPlayerNegative(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, -1)
	stb := ggpo.NewSyncTest(&session, 1, 8, 4, true)
	var handle ggpo.PlayerHandle
	err := stb.AddPlayer(&player, &handle)
	if err == nil {
		t.Errorf("There should be an error for adding a player with a negative number")
	}
}

func TestSyncTestBackendAddLocalInputError(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 1)
	stb := ggpo.NewSyncTest(&session, 1, 8, 4, true)
	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)
	err := stb.AddLocalInput(handle, []byte{1, 2, 3, 4}, 4)
	if err == nil {
		t.Errorf("There should be an error for adding local input when sync test isn't running yet.")
	}
}

func TestSyncTestBackendAddLocalInput(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 1)
	stb := ggpo.NewSyncTest(&session, 1, 8, 4, true)
	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)
	stb.Idle(0)
	err := stb.AddLocalInput(handle, []byte{1, 2, 3, 4}, 4)
	if err != nil {
		t.Errorf("There shouldn't be an error, adding local input should be successful.")
	}
}

func TestSyncTestBackendSyncInput(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 1)
	stb := ggpo.NewSyncTest(&session, 1, 8, 4, true)
	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)
	stb.Idle(0)
	inputBytes := []byte{1, 2, 3, 4}
	stb.AddLocalInput(handle, []byte{1, 2, 3, 4}, 4)
	var disconnectFlags int
	input, _ := stb.SyncInput(&disconnectFlags)
	got := input[0]
	want := inputBytes
	if !bytes.Equal(got, want) {
		t.Errorf("expected '%#v' but got '%#v'", want, got)
	}
}

func TestSyncTestBackendIncrementFramePanic(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 1)
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic wen trying to load a state that hadn't been saved")
		}
	}()
	for i := 0; i < checkDistance; i++ {
		stb.AdvanceFrame(ggpo.DefaultChecksum)
	}
}

func TestSyncTestBackendIncrementFrameCharacterization(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 1)
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)
	stb.Idle(0)
	inputBytes := []byte{1, 2, 3, 4}
	var disconnectFlags int
	for i := 0; i < checkDistance-1; i++ {
		stb.AddLocalInput(handle, inputBytes, 4)
		stb.SyncInput(&disconnectFlags)
		stb.AdvanceFrame(ggpo.DefaultChecksum)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic due to a SyncError")
		}
	}()
	stb.AdvanceFrame(ggpo.DefaultChecksum)

}

// Attempt to simulate STB workflow in a test harness. Always panics on check
// distance frame. Do not full understand why even though it works perfectly
// fine in real time.
func TestSyncTestBackendIncrementFrame(t *testing.T) {
	session := mocks.NewFakeSession()
	player := ggpo.NewLocalPlayer(20, 1)
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)
	inputBytes := []byte{1, 2, 3, 4}
	var disconnectFlags int
	var result error
	for i := 0; i < checkDistance-1; i++ {
		stb.Idle(0)
		result = stb.AddLocalInput(handle, inputBytes, 4)
		if result == nil {
			_, result = stb.SyncInput(&disconnectFlags)
			if result == nil {
				stb.AdvanceFrame(ggpo.DefaultChecksum)
			}
		}
	}

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic due to a SyncError")
		}
	}()
	stb.AdvanceFrame(ggpo.DefaultChecksum)
}

/*Again, WIP, I don't know how to test that this is working, but it is. */
func TestSyncTestBackendChecksumCheck(t *testing.T) {
	session := mocks.NewFakeSessionWithBackend()
	var stb ggpo.SyncTest
	session.SetBackend(&stb)
	player := ggpo.NewLocalPlayer(20, 1)
	checkDistance := 8
	stb = ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)

	var handle ggpo.PlayerHandle
	stb.AddPlayer(&player, &handle)
	inputBytes := []byte{1, 2, 3, 4}
	var disconnectFlags int
	var result error

	for i := 0; i < checkDistance+1; i++ {
		stb.Idle(0)
		result = stb.AddLocalInput(handle, inputBytes, 4)
		if result == nil {
			vals, result := stb.SyncInput(&disconnectFlags)
			if result == nil {
				session.Game.UpdateByInputs(vals)
				stb.AdvanceFrame(ggpo.DefaultChecksum)
			}
		}
	}
}

func TestSyncTestBackendMultiplePlayers(t *testing.T) {
	session := mocks.NewFakeSessionWithBackend()
	var stb ggpo.SyncTest
	session.SetBackend(&stb)
	player1 := ggpo.NewLocalPlayer(20, 1)
	player2 := ggpo.NewLocalPlayer(20, 2)
	checkDistance := 8
	stb = ggpo.NewSyncTest(&session, 2, checkDistance, 2, true)

	var handle1 ggpo.PlayerHandle
	stb.AddPlayer(&player1, &handle1)

	var handle2 ggpo.PlayerHandle
	stb.AddPlayer(&player2, &handle2)

	ib1 := []byte{2, 4}
	ib2 := []byte{1, 3}
	var disconnectFlags int
	var result error

	res1 := []float64{0, 0}
	res2 := []float64{0, 0}
	for i := 0; i < checkDistance+1; i++ {
		res1[0] += float64(ib1[0])
		res1[1] += float64(ib1[1])
		res2[0] += float64(ib2[0])
		res2[1] += float64(ib2[1])
	}

	for i := 0; i < checkDistance+1; i++ {
		stb.Idle(0)
		result = stb.AddLocalInput(handle1, ib1, 2)
		if result == nil {
			result = stb.AddLocalInput(handle2, ib2, 2)
		}
		if result == nil {
			vals, result := stb.SyncInput(&disconnectFlags)
			if result == nil {
				session.Game.UpdateByInputs(vals)
				stb.AdvanceFrame(ggpo.DefaultChecksum)
			}
		}
	}
	if session.Game.Players[0].X != res1[0] || session.Game.Players[0].Y != res1[1] ||
		session.Game.Players[1].X != res2[0] || session.Game.Players[1].Y != res2[1] {
		t.Errorf("Invalid result in 2 Player SyncTest Session")
	}
}

// Unsupported functions
func TestSyncTestBackendDissconnectPlayerError(t *testing.T) {
	session := mocks.NewFakeSession()
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	err := stb.DisconnectPlayer(ggpo.PlayerHandle(1))
	if err == nil {
		t.Errorf("The code did not error when using an unsupported Feature.")
	}
}

func TestSyncTestBackendGetNetworkStatsError(t *testing.T) {
	session := mocks.NewFakeSession()
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	_, err := stb.GetNetworkStats(ggpo.PlayerHandle(1))
	if err == nil {
		t.Errorf("The code did not error when using an unsupported Feature.")
	}
}

func TestSyncTestBackendSetFrameDelayError(t *testing.T) {
	session := mocks.NewFakeSession()
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	err := stb.SetFrameDelay(ggpo.PlayerHandle(1), 20)
	if err == nil {
		t.Errorf("The code did not error when using an unsupported Feature.")
	}
}

func TestSyncTestBackendSetDisconnectTimeoutError(t *testing.T) {
	session := mocks.NewFakeSession()
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	err := stb.SetDisconnectTimeout(20)
	if err == nil {
		t.Errorf("The code did not error when using an unsupported Feature.")
	}
}

func TestSyncTestBackendSetDisconnectNotifyStartError(t *testing.T) {
	session := mocks.NewFakeSession()
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	err := stb.SetDisconnectNotifyStart(20)
	if err == nil {
		t.Errorf("The code did not error when using an unsupported Feature.")
	}
}

func TestSyncTestBackendCloseError(t *testing.T) {
	session := mocks.NewFakeSession()
	checkDistance := 8
	stb := ggpo.NewSyncTest(&session, 1, checkDistance, 4, true)
	err := stb.Close()
	if err == nil {
		t.Errorf("The code did not error when using an unsupported Feature.")
	}
}
