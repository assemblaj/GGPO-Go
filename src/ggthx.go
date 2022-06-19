package ggthx

func GGTHXStartSession(
	session GGTHXSession,
	cb GGTHXSessionCallbacks,
	game string,
	numPlayers int,
	inputSize int,
	localPort uint16) GGTHXErrorCode {
	return GGTHX_OK
}

func GGTHXAddPlayer(ggthx GGTHXSession,
	player *GGTHXPlayer,
	handle *GGTHXPlayerHandle) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.AddPlayer(player, handle)
}

func GGTHXStartSyncTest(
	cb *GGTHXSessionCallbacks,
	game string,
	numPlayers int,
	inputSize int,
	frames int) (*SyncTestBackend, GGTHXErrorCode) {
	ggthx := NewSyncTestBackend(cb, game, numPlayers, frames, inputSize)
	return &ggthx, GGTHX_OK
}

func GGTHXSetFrameDelay(ggthx GGTHXSession,
	player GGTHXPlayerHandle,
	frameDelay int) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.SetFrameDelay(player, frameDelay)
}

func GGTHXIdle(ggthx GGTHXSession, timeout int) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.DoPoll(timeout)
}

func GGTHXAddLocalInput(ggthx GGTHXSession, player GGTHXPlayerHandle, values []byte, size int) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.AddLocalInput(player, values, size)
}

func GGTHXSynchronizeInput(ggthx GGTHXSession, disconnectFlags *int) ([]byte, GGTHXErrorCode) {
	if ggthx == nil {
		return nil, GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.SyncInput(disconnectFlags)
}

func GGTHXDisconnectPlayer(ggthx GGTHXSession, player GGTHXPlayerHandle) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.DisconnectPlayer(&player)
}

func GGTHXAdvanceFrame(ggthx GGTHXSession) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.IncrementFrame()
}

func GGTHXClientChat(ggthx GGTHXSession, text string) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.Chat(text)
}

func GGTHXGetNetworkStats(ggthx GGTHXSession, player GGTHXPlayerHandle, stats *GGTHXNetworkStats) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.GetNetworkStats(stats, player)
}

func GGTHXCloseSession(ggthx GGTHXSession) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	ggthx.Close()
	return GGTHX_OK
}

func GGTHXSetDisconnectTimeout(ggthx GGTHXSession, timeout int) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.SetDisconnectTimeout(timeout)
}

func GGTHXSetDisconnectNotifyStart(ggthx GGTHXSession, timeout int) GGTHXErrorCode {
	if ggthx == nil {
		return GGTHX_ERRORCODE_INVALID_SESSION
	}
	return ggthx.SetDisconnectNotifyStart(timeout)
}

func GGTHXSartSpectating(cb *GGTHXSessionCallbacks,
	gameName string, lcaolPort int, numPlayers int, inputSize int, hostIp string, hostPort int) (*SpectatorBackend, GGTHXErrorCode) {
	ggthx := NewSpectatorBackend(cb, gameName, lcaolPort, numPlayers, inputSize, hostIp, hostPort)
	return &ggthx, GGTHX_OK
}
