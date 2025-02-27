package protocol

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/assemblaj/ggpo/internal/buffer"
	"github.com/assemblaj/ggpo/internal/input"
	"github.com/assemblaj/ggpo/internal/messages"
	"github.com/assemblaj/ggpo/internal/polling"
	"github.com/assemblaj/ggpo/internal/sync"
	"github.com/assemblaj/ggpo/internal/util"
	"github.com/assemblaj/ggpo/transport"
)

const (
	// UDPHeaderSize is the size of the IP + UDP headers.
	UDPHeaderSize          = 28
	NumSyncPackets         = 5
	SyncRetryInterval      = 2000
	SyncFirstRetryInterval = 500
	RunningRetryInterval   = 200
	KeepAliveInterval      = 200
	QualityReportInterval  = 1000
	NetworkStatsInterval   = 1000
	UDPShutdownTimer       = 5000
	MaxSeqDistance         = 1 << 15
)

type UdpProtocol struct {
	stats UdpProtocolStats // may not need these
	event UdpProtocolEvent //

	// Network transmission information
	connection        transport.Connection
	peerAddress       string
	peerPort          int
	magicNumber       uint16
	queue             int
	remoteMagicNumber uint16
	connected         bool
	sendLatency       int64
	ooPercent         int
	ooPacket          OoPacket
	sendQueue         buffer.RingBuffer[QueueEntry]

	// Stats
	roundTripTime  int64
	packetsSent    int
	bytesSent      int
	kbpsSent       int
	statsStartTime int64

	// The State Machine
	localConnectStatus *[]messages.UdpConnectStatus
	peerConnectStatus  []messages.UdpConnectStatus
	currentState       UdpProtocolState
	state              UdpProtocolStateInfo

	// Fairness
	localFrameAdvantage  float32
	remoteFrameAdvantage float32

	// Packet Loss
	pendingOutput         buffer.RingBuffer[input.GameInput]
	lastRecievedInput     input.GameInput
	lastSentInput         input.GameInput
	lastAckedInput        input.GameInput
	lastSendTime          int64
	lastRecvTime          int64
	shutdownTimeout       int64
	disconnectEventSent   bool
	disconnectTimeout     int64
	disconnectNotifyStart int64
	disconnectNotifySent  bool

	nextSendSeq uint16
	nextRecvSeq uint16

	// Rift synchronization
	timesync sync.TimeSync

	// Event Queue
	eventQueue buffer.RingBuffer[UdpProtocolEvent]

	RemoteChecksumsThisFrame util.OrderedMap[int, uint32]
	RemoteChecksums          util.OrderedMap[int, uint32]
}

type NetworkStats struct {
	Network  NetworkNetworkStats
	Timesync NetworkTimeSyncStats
}

type NetworkNetworkStats struct {
	SendQueueLen int
	RecvQueueLen int
	Ping         int64
	KbpsSent     int
}
type NetworkTimeSyncStats struct {
	LocalFramesBehind     float32
	RemoteFramesBehind    float32
	AvgLocalFramesBehind  float32
	AvgRemoteFramesBehind float32
}

type UdpProtocolStats struct {
	ping                int
	remoteFrameAdvtange int
	localFrameAdvantage int
	sendQueueLen        int
	udp                 transport.UdpStats
}

type UdpProtocolEvent struct {
	eventType         UdpProtocolEventType
	Input             input.GameInput // for Input message
	Total             int             // for synchronizing
	Count             int             //
	DisconnectTimeout int             // network interrupted
}

func (upe UdpProtocolEvent) Type() UdpProtocolEventType {
	return upe.eventType
}

// for logging purposes only
func (upe UdpProtocolEvent) String() string {
	str := "(event:"
	switch upe.eventType {
	case UnknownEvent:
		str += "Unknown"
		break
	case ConnectedEvent:
		str += "Connected"
		break
	case SynchronizingEvent:
		str += "Synchronizing"
		break
	case SynchronziedEvent:
		str += "Synchronized"
		break
	case InputEvent:
		str += "Input"
		break
	case DisconnectedEvent:
		str += "Disconnected"
		break
	case NetworkInterruptedEvent:
		str += "NetworkInterrupted"
		break
	case NetworkResumedEvent:
		str += "NetworkResumed"
		break
	}
	str += ").\n"
	return str
}

type UdpProtocolEventType int

const (
	UnknownEvent UdpProtocolEventType = iota - 1
	ConnectedEvent
	SynchronizingEvent
	SynchronziedEvent
	InputEvent
	DisconnectedEvent
	NetworkInterruptedEvent
	NetworkResumedEvent
)

type UdpProtocolState int

const (
	SyncingState UdpProtocolState = iota
	SynchronziedState
	RunningState
	DisconnectedState
)

type UdpProtocolStateInfo struct {
	roundTripRemaining uint32 // sync
	random             uint32

	lastQualityReportTime    int64 // running
	lastNetworkStatsInterval int64
	lastInputPacketRecvTime  int64
}

type QueueEntry struct {
	queueTime int64
	destIp    string
	msg       messages.UDPMessage
	destPort  int
}

func (q QueueEntry) String() string {
	return fmt.Sprintf("Entry : queueTime %d destIp %s msg %s", q.queueTime, q.destIp, q.msg)
}

func NewQueEntry(time int64, destIp string, destPort int, m messages.UDPMessage) QueueEntry {
	return QueueEntry{
		queueTime: time,
		destIp:    destIp,
		destPort:  destPort,
		msg:       m,
	}
}

type OoPacket struct {
	sendTime int64
	destIp   string
	msg      messages.UDPMessage
}

func NewUdpProtocol(connection transport.Connection, queue int, ip string, port int, status *[]messages.UdpConnectStatus) UdpProtocol {
	var magicNumber uint16
	for {
		magicNumber = uint16(rand.Int())
		if magicNumber != 0 {
			break
		}
	}
	peerConnectStatus := make([]messages.UdpConnectStatus, messages.UDPMsgMaxPlayers)
	for i := 0; i < len(peerConnectStatus); i++ {
		peerConnectStatus[i].LastFrame = -1
	}
	lastSentInput, _ := input.NewGameInput(-1, nil, 1)
	lastRecievedInput, _ := input.NewGameInput(-1, nil, 1)
	lastAckedInput, _ := input.NewGameInput(-1, nil, 1)

	protocol := UdpProtocol{
		connection:               connection,
		queue:                    queue,
		localConnectStatus:       status,
		peerConnectStatus:        peerConnectStatus,
		peerAddress:              ip,
		peerPort:                 port,
		magicNumber:              magicNumber,
		pendingOutput:            buffer.NewRingBuffer[input.GameInput](64),
		sendQueue:                buffer.NewRingBuffer[QueueEntry](64),
		eventQueue:               buffer.NewRingBuffer[UdpProtocolEvent](64),
		timesync:                 sync.NewTimeSync(),
		lastSentInput:            lastSentInput,
		lastRecievedInput:        lastRecievedInput,
		lastAckedInput:           lastAckedInput,
		RemoteChecksums:          util.NewOrderedMap[int, uint32](16),
		RemoteChecksumsThisFrame: util.NewOrderedMap[int, uint32](16),
	}
	//poll.RegisterLoop(&protocol, nil)
	return protocol
}

func (u *UdpProtocol) StartPollLoop() {
	u.RemoteChecksumsThisFrame.Clear()
}

func (u *UdpProtocol) EndPollLoop() {
	if u.RemoteChecksumsThisFrame.Len() > 0 {
		highestFrameNum := u.RemoteChecksumsThisFrame.Greatest()
		u.RemoteChecksums.Set(highestFrameNum.Key, highestFrameNum.Value)
	}
}

func (u *UdpProtocol) SetIncomingRemoteChecksum(frame int, checksum uint32) {
	u.RemoteChecksumsThisFrame.Set(frame, checksum)
}

func (u *UdpProtocol) SetFrameDelay(delay int) {
	u.timesync.SetFrameDelay(delay)
}

func (u *UdpProtocol) RemoteFrameDelay() int {
	return u.timesync.RemoteFrameDelay
}

func (u *UdpProtocol) OnLoopPoll(timeFunc polling.FuncTimeType) bool {

	// originally was if !udp
	if u.connection == nil {
		return true
	}
	now := timeFunc()

	var nextInterval int64

	err := u.PumpSendQueue()
	if err != nil {
		panic(err)
	}

	switch u.currentState {
	case SyncingState:
		if int(u.state.roundTripRemaining) == NumSyncPackets {
			nextInterval = SyncFirstRetryInterval
		} else {
			nextInterval = SyncRetryInterval
		}
		if u.lastSendTime > 0 && u.lastSendTime+nextInterval < now {
			util.Log.Printf("No luck syncing after %d ms... Re-queueing sync packet.\n", nextInterval)
			u.SendSyncRequest()
		}
		break
	case RunningState:
		if u.state.lastInputPacketRecvTime == 0 || u.state.lastInputPacketRecvTime+RunningRetryInterval > now {
			util.Log.Printf("Haven't exchanged packets in a while (last received:%d  last sent:%d).  Resending.\n",
				u.lastRecievedInput.Frame, u.lastSentInput.Frame)
			err := u.SendPendingOutput()
			if err != nil {
				panic(err)
			}
			u.state.lastInputPacketRecvTime = now
		}

		//if (!u.State.running.last_quality_report_time || _state.running.last_quality_report_time + QUALITY_REPORT_INTERVAL < now) {
		if u.state.lastQualityReportTime == 0 || uint32(u.state.lastQualityReportTime)+uint32(QualityReportInterval) < uint32(now) {
			msg := messages.NewUDPMessage(messages.QualityReportMsg)
			qualityReport := msg.(*messages.QualityReportPacket)
			qualityReport.Ping = uint64(time.Now().UnixMilli())
			qualityReport.FrameAdvantage = int8(util.Min(255.0, u.timesync.LocalAdvantage()*10))
			u.SendMsg(qualityReport)
			u.state.lastQualityReportTime = now
		}

		if u.state.lastNetworkStatsInterval == 0 || u.state.lastNetworkStatsInterval+NetworkStatsInterval < now {
			u.UpdateNetworkStats()
			u.state.lastNetworkStatsInterval = now
		}

		if u.lastSendTime > 0 && u.lastSendTime+KeepAliveInterval < now {
			util.Log.Println("Sending keep alive packet")
			msg := messages.NewUDPMessage(messages.KeepAliveMsg)
			u.SendMsg(msg)
		}

		if u.disconnectTimeout > 0 && u.disconnectNotifyStart > 0 &&
			!u.disconnectNotifySent && (u.lastRecvTime+u.disconnectNotifyStart < now) {
			util.Log.Printf("Endpoint has stopped receiving packets for %d ms.  Sending notification.\n", u.disconnectNotifyStart)
			e := UdpProtocolEvent{
				eventType: NetworkInterruptedEvent}
			e.DisconnectTimeout = int(u.disconnectTimeout) - int(u.disconnectNotifyStart)
			u.QueueEvent(&e)
			u.disconnectNotifySent = true
		}

		if u.disconnectTimeout > 0 && (u.lastRecvTime+u.disconnectTimeout < now) {
			if !u.disconnectEventSent {
				util.Log.Printf("Endpoint has stopped receiving packets for %d ms.  Disconnecting.\n",
					u.disconnectTimeout)
				u.QueueEvent(&UdpProtocolEvent{
					eventType: DisconnectedEvent})
				u.disconnectEventSent = true
			}
		}
		break
	case DisconnectedState:
		if u.shutdownTimeout < now {
			util.Log.Printf("Shutting down udp connection.\n")
			u.connection = nil
			u.shutdownTimeout = 0
		}
	}
	return true
}

// all this method did, and all bitvector did, in the original
// was incode input as bits, etc
// go globs can do a lot of that for us, so i've forgone much of that logic
// https://github.com/pond3r/ggpo/blob/7ddadef8546a7d99ff0b3530c6056bc8ee4b9c0a/src/lib/ggpo/network/udp_proto.cpp#L111
func (u *UdpProtocol) SendPendingOutput() error {
	msg := messages.NewUDPMessage(messages.InputMsg)
	inputMsg := msg.(*messages.InputPacket)

	var j, offset int

	if u.pendingOutput.Size() > 0 {
		last := u.lastAckedInput
		// bits = msg.Input.Bits
		input, err := u.pendingOutput.Front()
		if err != nil {
			panic(err)
		}
		inputMsg.StartFrame = uint32(input.Frame)
		inputMsg.InputSize = uint8(input.Size)

		if !(last.Frame == -1 || last.Frame+1 == int(inputMsg.StartFrame)) {
			return errors.New("ggpo UdpProtocol SendPendingOutput: !((last.Frame == -1 || last.Frame+1 == int(msg.Input.StartFrame))) ")
		}

		for j = 0; j < u.pendingOutput.Size(); j++ {
			current, err := u.pendingOutput.Item(j)
			inputMsg.Checksum = current.Checksum
			if err != nil {
				panic(err)
			}
			inputMsg.Bits = append(inputMsg.Bits, current.Bits...)
			last = current // might get rid of this
			u.lastSentInput = current
		}
	} else {
		inputMsg.StartFrame = 0
		inputMsg.InputSize = 0
	}

	inputMsg.AckFrame = int32(u.lastRecievedInput.Frame)
	// msg.Input.NumBits = uint16(offset) caused major bug, numbits would always be 0
	// causing input events to never be recieved
	inputMsg.NumBits = uint16(inputMsg.InputSize)

	inputMsg.DisconectRequested = u.currentState == DisconnectedState

	if u.localConnectStatus != nil {
		inputMsg.PeerConnectStatus = make([]messages.UdpConnectStatus, len(*u.localConnectStatus))
		copy(inputMsg.PeerConnectStatus, *u.localConnectStatus)
	} else {
		inputMsg.PeerConnectStatus = make([]messages.UdpConnectStatus, messages.UDPMsgMaxPlayers)
	}

	// may not even need this.
	if offset >= messages.MaxCompressedBits {
		return errors.New("ggpo UdpProtocol SendPendingOutput: offset >= MaxCompressedBits")
	}

	u.SendMsg(inputMsg)
	return nil
}

func (u *UdpProtocol) SendInputAck() {
	msg := messages.NewUDPMessage(messages.InputAckMsg)
	inputAck := msg.(*messages.InputAckPacket)
	inputAck.AckFrame = int32(u.lastRecievedInput.Frame)
	u.SendMsg(inputAck)
}

func (u *UdpProtocol) GetEvent() (*UdpProtocolEvent, error) {
	if u.eventQueue.Size() == 0 {
		return nil, errors.New("ggpo UdpProtocol GetEvent:no events")
	}
	e, err := u.eventQueue.Front()
	if err != nil {
		panic(err)
	}
	err = u.eventQueue.Pop()
	if err != nil {
		panic(err)
	}
	return &e, nil
}

func (u *UdpProtocol) QueueEvent(evt *UdpProtocolEvent) {
	util.Log.Printf("Queueing event %s", *evt)
	err := u.eventQueue.Push(*evt)
	// if there's no more room left in the queue, make room.
	if err != nil {
		//u.eventQueue.Pop()
		//u.eventQueue.Push(*evt)
		panic(err)
	}
}

func (u *UdpProtocol) Disconnect() {
	u.currentState = DisconnectedState
	u.shutdownTimeout = time.Now().UnixMilli() + UDPShutdownTimer
}

func (u *UdpProtocol) SendSyncRequest() {
	u.state.random = uint32(rand.Int() & 0xFFFF)
	msg := messages.NewUDPMessage(messages.SyncRequestMsg)
	syncRequest := msg.(*messages.SyncRequestPacket)
	syncRequest.RandomRequest = u.state.random
	syncRequest.RemoteInputDelay = uint8(u.timesync.FrameDelay2)
	u.SendMsg(syncRequest)
}

func (u *UdpProtocol) SendMsg(msg messages.UDPMessage) {
	util.Log.Printf("In UdpProtocol send %s", msg)
	u.packetsSent++
	u.lastSendTime = time.Now().UnixMilli()
	u.bytesSent += msg.PacketSize()
	msg.SetHeader(u.magicNumber, u.nextSendSeq)
	u.nextSendSeq++
	if u.peerAddress == "" {
		panic("peerAdress empty, why?")
	}
	var err error
	err = u.sendQueue.Push(NewQueEntry(
		time.Now().UnixMilli(), u.peerAddress, u.peerPort, msg))
	if err != nil {
		panic(err)
	}

	err = u.PumpSendQueue()
	if err != nil {
		panic(err)
	}
}

func (u *UdpProtocol) OnInput(msg messages.UDPMessage, length int) (bool, error) {
	inputMessage := msg.(*messages.InputPacket)

	// If a disconnect is requested, go ahead and disconnect now.
	disconnectRequested := inputMessage.DisconectRequested
	if disconnectRequested {
		if u.currentState != DisconnectedState && !u.disconnectEventSent {
			util.Log.Printf("Disconnecting endpoint on remote request.\n")
			u.QueueEvent(&UdpProtocolEvent{
				eventType: DisconnectedEvent,
			})
			u.disconnectEventSent = true
		}
	} else {
		// update the peer connection status if this peer is still considered to be part
		// of the network
		remoteStatus := inputMessage.PeerConnectStatus
		for i := 0; i < len(u.peerConnectStatus); i++ {
			if remoteStatus[i].LastFrame < u.peerConnectStatus[i].LastFrame {
				return false, errors.New("ggpo UdpProtocol OnInput: remoteStatus[i].LastFrame < u.peerConnectStatus[i].LastFrame")
			}
			u.peerConnectStatus[i].Disconnected = u.peerConnectStatus[i].Disconnected || remoteStatus[i].Disconnected
			u.peerConnectStatus[i].LastFrame = util.Max(u.peerConnectStatus[i].LastFrame, remoteStatus[i].LastFrame)
		}
	}

	// Decompress the input (gob should already have done this)
	lastRecievedFrameNumber := u.lastRecievedInput.Frame

	currentFrame := inputMessage.StartFrame

	u.lastRecievedInput.Size = int(inputMessage.InputSize)
	if u.lastRecievedInput.Frame < 0 {
		u.lastRecievedInput.Frame = int(inputMessage.StartFrame) - 1
	}

	offset := 0

	for offset < len(inputMessage.Bits) {
		if currentFrame > uint32(u.lastRecievedInput.Frame+1) {
			return false, errors.New("ggpo UdpProtocol OnInput: currentFrame > uint32(u.lastRecievedInput.Frame + 1)")
		}
		useInputs := currentFrame == uint32(u.lastRecievedInput.Frame+1)
		if useInputs {
			if currentFrame != uint32(u.lastRecievedInput.Frame)+1 {
				return false, errors.New("ggpo UdpProtocol OnInput: currentFrame != uint32(u.lastRecievedInput.Frame) +1")
			}
			u.lastRecievedInput.Bits = inputMessage.Bits[offset : offset+int(inputMessage.InputSize)]
			u.lastRecievedInput.Frame = int(currentFrame)
			u.lastRecievedInput.Checksum = inputMessage.Checksum
			evt := UdpProtocolEvent{
				eventType: InputEvent,
				Input:     u.lastRecievedInput,
			}
			u.state.lastInputPacketRecvTime = time.Now().UnixMilli()
			util.Log.Printf("Sending frame %d to emu queue %d.\n", u.lastRecievedInput.Frame, u.queue)
			u.QueueEvent(&evt)
			u.SendInputAck()
		} else {
			util.Log.Printf("Skipping past frame:(%d) current is %d.\n", currentFrame, u.lastRecievedInput.Frame)

		}
		offset += int(inputMessage.InputSize)
		currentFrame++
	}

	if u.lastRecievedInput.Frame < lastRecievedFrameNumber {
		return false, errors.New("ggpo UdpProtocol OnInput: u.lastRecievedInput.Frame < lastRecievedFrameNumber")
	}

	// Get rid of our buffered input
	for u.pendingOutput.Size() > 0 {
		input, err := u.pendingOutput.Front()
		if err != nil {
			panic(err)
		}
		if int32(input.Frame) < inputMessage.AckFrame {
			util.Log.Printf("Throwing away pending output frame %d\n", input.Frame)
			u.lastAckedInput = input
			err := u.pendingOutput.Pop()
			if err != nil {
				panic(err)
			}
		} else {
			break
		}
	}
	return true, nil
}

func (u *UdpProtocol) OnInputAck(msg messages.UDPMessage, len int) (bool, error) {
	inputAck := msg.(*messages.InputAckPacket)
	// Get rid of our buffered input
	for u.pendingOutput.Size() > 0 {
		input, err := u.pendingOutput.Front()
		if err != nil {
			panic(err)
		}
		if int32(input.Frame) < inputAck.AckFrame {
			util.Log.Printf("Throwing away pending output frame %d\n", input.Frame)
			u.lastAckedInput = input
			err = u.pendingOutput.Pop()
			if err != nil {
				panic(err)
			}
		} else {
			break
		}
	}
	return true, nil
}

func (u *UdpProtocol) OnQualityReport(msg messages.UDPMessage, len int) (bool, error) {
	qualityReport := msg.(*messages.QualityReportPacket)
	reply := messages.NewUDPMessage(messages.QualityReplyMsg)
	replyPacket := reply.(*messages.QualityReplyPacket)
	replyPacket.Pong = qualityReport.Ping
	u.SendMsg(replyPacket)

	u.remoteFrameAdvantage = float32(qualityReport.FrameAdvantage) / 10.0
	return true, nil
}

func (u *UdpProtocol) OnQualityReply(msg messages.UDPMessage, len int) (bool, error) {
	qualityReply := msg.(*messages.QualityReplyPacket)
	u.roundTripTime = time.Now().UnixMilli() - int64(qualityReply.Pong)
	return true, nil
}

func (u *UdpProtocol) OnKeepAlive(msg messages.UDPMessage, len int) (bool, error) {
	return true, nil
}

func (u *UdpProtocol) GetNetworkStats() NetworkStats {
	s := NetworkStats{}
	s.Network.Ping = u.roundTripTime
	s.Network.SendQueueLen = u.pendingOutput.Size()
	s.Network.KbpsSent = u.kbpsSent
	s.Timesync.RemoteFramesBehind = u.timesync.RemoteAdvantage()
	s.Timesync.LocalFramesBehind = u.timesync.LocalAdvantage()
	s.Timesync.AvgLocalFramesBehind = u.timesync.AvgLocalAdvantageSinceStart()
	s.Timesync.AvgRemoteFramesBehind = u.timesync.AvgRemoteAdvantageSinceStart()
	return s
}

func (u *UdpProtocol) SetLocalFrameNumber(localFrame int) {
	remoteFrame := float32(int64(u.lastRecievedInput.Frame) + (u.roundTripTime * 60.0 / 2000.0))
	u.localFrameAdvantage = ((remoteFrame - float32(localFrame)) - float32(u.timesync.FrameDelay2))
}

func (u *UdpProtocol) RecommendFrameDelay() float32 {
	return u.timesync.ReccomendFrameWaitDuration(false)
}

func (u *UdpProtocol) SetDisconnectTimeout(timeout int) {
	u.disconnectTimeout = int64(timeout)
}

func (u *UdpProtocol) SetDisconnectNotifyStart(timeout int) {
	u.disconnectNotifyStart = int64(timeout)
}

func (u *UdpProtocol) PumpSendQueue() error {
	var entry QueueEntry
	var err error

	for !u.sendQueue.Empty() {
		entry, err = u.sendQueue.Front()
		if err != nil {
			panic(err)
		}

		if u.sendLatency > 0 {
			jitter := (u.sendLatency * 2 / 3) + ((rand.Int63() % u.sendLatency) / 3)
			if time.Now().UnixMilli() < entry.queueTime+jitter {
				break
			}
		}
		if u.ooPercent > 0 && u.ooPacket.msg == nil && ((rand.Int() % 100) < u.ooPercent) {
			delay := rand.Int63() % (u.sendLatency*10 + 1000)
			util.Log.Printf("creating rogue oop (seq: %d  delay: %d)\n",
				entry.msg.Header().SequenceNumber, delay)
			u.ooPacket.sendTime = time.Now().UnixMilli() + delay
			u.ooPacket.msg = entry.msg
			u.ooPacket.destIp = entry.destIp
		} else {
			if entry.destIp == "" {
				return errors.New("ggpo UdpProtocol PumpSendQueue: entry.destIp == \"\"")
			}
			u.connection.SendTo(entry.msg, entry.destIp, entry.destPort)
			// would delete the udpmsg here
		}
		err := u.sendQueue.Pop()
		if err != nil {
			panic(err)
		}
	}
	if u.ooPacket.msg != nil && u.ooPacket.sendTime < time.Now().UnixMilli() {
		util.Log.Printf("sending rogue oop!")
		u.connection.SendTo(u.ooPacket.msg, u.peerAddress, u.peerPort)
		u.ooPacket.msg = nil
	}
	return nil
}

func (u *UdpProtocol) ClearSendQueue() {
	for !u.sendQueue.Empty() {
		// i'd manually delete the QueueEntry in a language where I could
		err := u.sendQueue.Pop()
		if err != nil {
			panic(err)
		}
	}
}

// going to call deletes close
func (u *UdpProtocol) Close() {
	u.ClearSendQueue()
	if u.connection != nil {
		u.connection.Close()
	}
}

func (u *UdpProtocol) HandlesMsg(ipAddress string, port int) bool {
	if u.connection == nil {
		return false
	}
	return u.peerAddress == ipAddress && u.peerPort == port
}

func (u *UdpProtocol) SendInput(input *input.GameInput) {
	if u.connection != nil {
		if u.currentState == RunningState {
			// check to see if this is a good time to adjust for the rift
			u.timesync.AdvanceFrames(input, u.localFrameAdvantage, u.remoteFrameAdvantage)

			// Save this input packet.
			err := u.pendingOutput.Push(*input)
			// if for whatever reason the capacity is full, pop off the end of the buffer and try again
			if err != nil {
				//u.pendingOutput.Pop()
				//u.pendingOutput.Push(*input)
				panic(err)
			}
		}
		err := u.SendPendingOutput()
		if err != nil {
			panic(err)
		}
	}
}

func (u *UdpProtocol) UpdateNetworkStats() {
	now := time.Now().UnixMilli()
	if u.statsStartTime == 0 {
		u.statsStartTime = now
	}

	totalBytesSent := u.bytesSent + (UDPHeaderSize * u.packetsSent)
	seconds := float64(now-u.statsStartTime) / 1000.0
	bps := float64(totalBytesSent) / seconds
	udpOverhead := float64(100.0 * (float64(UDPHeaderSize * u.packetsSent)) / float64(u.bytesSent))
	u.kbpsSent = int(bps / 1024)

	util.Log.Printf("Network Stats -- Bandwidth: %.2f KBps Packets Sent: %5d (%.2f pps) KB Sent: %.2f UDP Overhead: %.2f %%.\n",
		float64(u.kbpsSent),
		u.packetsSent,
		float64(u.packetsSent*1000)/float64(now-u.statsStartTime),
		float64(totalBytesSent/1024.0),
		udpOverhead)
}

func (u *UdpProtocol) Synchronize() {
	if u.connection != nil {
		u.currentState = SyncingState
		u.state.roundTripRemaining = uint32(NumSyncPackets)
		u.SendSyncRequest()
	}
}

func (u *UdpProtocol) GetPeerConnectStatus(id int, frame *int32) bool {
	*frame = u.peerConnectStatus[id].LastFrame
	// !u.peerConnectStatus[id].Disconnected from the C/++ world
	return !u.peerConnectStatus[id].Disconnected
}

func (u *UdpProtocol) OnInvalid(msg messages.UDPMessage, len int) (bool, error) {
	//  Assert(false) // ? ASSERT(FALSE && "Invalid msg in UdpProtocol");
	// ah
	util.Log.Printf("Invalid msg in UdpProtocol ")
	return false, errors.New("ggpo UdpProtocol OnInvalid: invalid msg in UdpProtocol")
}

func (u *UdpProtocol) OnSyncRequest(msg messages.UDPMessage, len int) (bool, error) {
	request := msg.(*messages.SyncRequestPacket)
	reply := messages.NewUDPMessage(messages.SyncReplyMsg)
	syncReply := reply.(*messages.SyncReplyPacket)
	syncReply.RandomReply = request.RandomRequest
	u.timesync.RemoteFrameDelay = int(request.RemoteInputDelay)
	u.SendMsg(syncReply)
	return true, nil
}

func (u *UdpProtocol) OnMsg(msg messages.UDPMessage, length int) {
	handled := false
	var err error
	type UdpProtocolDispatchFunc func(msg messages.UDPMessage, length int) (bool, error)

	table := []UdpProtocolDispatchFunc{
		u.OnInvalid,
		u.OnSyncRequest,
		u.OnSyncReply,
		u.OnInput,
		u.OnQualityReport,
		u.OnQualityReply,
		u.OnKeepAlive,
		u.OnInputAck}

	// filter out messages that don't match what we expect
	seq := msg.Header().SequenceNumber
	if msg.Header().HeaderType != uint8(messages.SyncRequestMsg) && msg.Header().HeaderType != uint8(messages.SyncReplyMsg) {
		if msg.Header().Magic != u.remoteMagicNumber {
			util.Log.Printf("recv rejecting %s", msg)
			return
		}

		// filer out out-of-order packets
		skipped := seq - u.nextRecvSeq
		if skipped > uint16(MaxSeqDistance) {
			util.Log.Printf("dropping out of order packet (seq: %d, last seq:%d)\n", seq, u.nextRecvSeq)
			return
		}
	}

	u.nextRecvSeq = seq
	util.Log.Printf("recv %s on queue %d\n", msg, u.queue)
	if int(msg.Header().HeaderType) >= len(table) {
		u.OnInvalid(msg, length)
	} else {
		handled, err = table[int(msg.Header().HeaderType)](msg, length)
	}
	if err != nil {
		panic(err)
	}

	if handled {
		u.lastRecvTime = time.Now().UnixMilli()
		if u.disconnectNotifySent && u.currentState == RunningState {
			u.QueueEvent(
				&UdpProtocolEvent{
					eventType: NetworkResumedEvent,
				})
			u.disconnectNotifySent = false
		}
	}
}

func (u *UdpProtocol) OnSyncReply(msg messages.UDPMessage, length int) (bool, error) {
	syncReply := msg.(*messages.SyncReplyPacket)
	if u.currentState != SyncingState {
		util.Log.Println("Ignoring SyncReply while not synching.")
		return msg.Header().Magic == u.remoteMagicNumber, nil
	}

	if syncReply.RandomReply != u.state.random {
		util.Log.Printf("sync reply %d != %d.  Keep looking...\n",
			syncReply.RandomReply, u.state.random)
		return false, nil
	}

	if !u.connected {
		u.QueueEvent(&UdpProtocolEvent{
			eventType: ConnectedEvent})
		u.connected = true
	}

	util.Log.Printf("Checking sync state (%d round trips remaining).\n", u.state.roundTripRemaining)
	u.state.roundTripRemaining--
	if u.state.roundTripRemaining == 0 {
		util.Log.Printf("Synchronized!\n")
		u.QueueEvent(&UdpProtocolEvent{
			eventType: SynchronziedEvent,
		})
		u.currentState = RunningState
		u.lastRecievedInput.Frame = -1
		u.remoteMagicNumber = msg.Header().Magic
	} else {
		evt := UdpProtocolEvent{
			eventType: SynchronizingEvent,
		}
		evt.Total = NumSyncPackets
		evt.Count = NumSyncPackets - int(u.state.roundTripRemaining)
		u.QueueEvent(&evt)
		u.SendSyncRequest()
	}

	return true, nil
}

func (u *UdpProtocol) IsInitialized() bool {
	return u.connection != nil
}

func (u *UdpProtocol) IsSynchronized() bool {
	return u.currentState == RunningState
}

func (u *UdpProtocol) IsRunning() bool {
	return u.currentState == RunningState
}
