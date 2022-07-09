package ggthx

import (
	"errors"
	"fmt"
	"unsafe"
)

const (
	MaxCompressedBits = 4096
	UDPMsgMaxPlayers  = 4
)

// Bad bad bad. Want to find way around this. Giga packet.
// Original used Union
type UdpMsg struct {
	Header        UdpHeader
	MessageType   UdpMsgType
	SyncRequest   UdpSyncRequestPacket
	SyncReply     UdpSyncReplyPacket
	QualityReport UdpQualityReportPacket
	QualityReply  UdpQualityReplyPacket
	Input         UdpInputPacket
	InputAck      UdpInputAckPacket
}

type UdpMsgType int

const (
	InvalidMsg UdpMsgType = iota
	SyncRequestMsg
	SyncReplyMsg
	InputMsg
	QualityReportMsg
	QualityReplyMsg
	KeepAliveMsg
	InputAckMsg
)

type UdpConnectStatus struct {
	Disconnected bool
	LastFrame    int
}

type UdpHeader struct {
	Magic          uint16
	SequenceNumber uint16
	HeaderType     uint8
}

type UdpPacket struct {
	msg *UdpMsg
	len int
}

type UdpSyncRequestPacket struct {
	RandomRequest  uint32
	RemoteMagic    uint16
	RemoteEndpoint uint8
}

type UdpSyncReplyPacket struct {
	RandomReply uint32
}

type UdpQualityReportPacket struct {
	FrameAdvantage int8
	Ping           uint32
}

type UdpQualityReplyPacket struct {
	Pong uint32
}

type UdpInputPacket struct {
	PeerConnectStatus []UdpConnectStatus
	StartFrame        uint32

	DisconectRequested bool
	AckFrame           int

	NumBits     uint16
	InputSize   uint8
	Bits        [][]byte
	IsSpectator bool
}

type UdpInputAckPacket struct {
	AckFrame int
}

func NewUdpMsg(t UdpMsgType) UdpMsg {
	header := UdpHeader{HeaderType: uint8(t)}
	messageType := t
	var msg UdpMsg
	switch t {
	case SyncRequestMsg:
		msg = UdpMsg{
			Header:      header,
			MessageType: messageType,
			SyncRequest: UdpSyncRequestPacket{}}
	case SyncReplyMsg:
		msg = UdpMsg{
			Header:      header,
			MessageType: messageType,
			SyncReply:   UdpSyncReplyPacket{}}
	case QualityReportMsg:
		msg = UdpMsg{
			Header:        header,
			MessageType:   messageType,
			QualityReport: UdpQualityReportPacket{}}
	case QualityReplyMsg:
		msg = UdpMsg{
			Header:       header,
			MessageType:  messageType,
			QualityReply: UdpQualityReplyPacket{}}
	case InputAckMsg:
		msg = UdpMsg{
			Header:      header,
			MessageType: messageType,
			InputAck:    UdpInputAckPacket{}}
	case InputMsg:
		msg = UdpMsg{
			Header:      header,
			MessageType: messageType,
			Input:       UdpInputPacket{}}
	case KeepAliveMsg:
		fallthrough
	default:
		msg = UdpMsg{
			Header:      header,
			MessageType: messageType}

	}
	return msg
}

//func (u *UdpMsg) BuildQualityReply(pong int) {
//	u.qualityReply.pong = uint32(pong)
//}

func (u *UdpMsg) PacketSize() int {
	size, err := u.PaylaodSize()
	//Unknown Packet type somehow
	if err != nil {
		// Send size of whole object
		//return int(unsafe.Sizeof(u))
		panic(err)
	}
	return int(unsafe.Sizeof(u.Header)) + size
}

func (u *UdpMsg) PaylaodSize() (int, error) {
	var size int

	switch UdpMsgType(u.Header.HeaderType) {
	case SyncRequestMsg:
		return int(unsafe.Sizeof(u.SyncRequest)), nil
	case SyncReplyMsg:
		return int(unsafe.Sizeof(u.SyncReply)), nil
	case QualityReportMsg:
		return int(unsafe.Sizeof(u.QualityReport)), nil
	case QualityReplyMsg:
		return int(unsafe.Sizeof(u.QualityReply)), nil
	case InputAckMsg:
		return int(unsafe.Sizeof(u.InputAck)), nil
	case KeepAliveMsg:
		return 0, nil
	case InputMsg:
		for _, s := range u.Input.PeerConnectStatus {
			size += int(unsafe.Sizeof(s))
		}
		size += int(unsafe.Sizeof(u.Input.StartFrame))
		size += int(unsafe.Sizeof(u.Input.DisconectRequested))
		size += int(unsafe.Sizeof(u.Input.AckFrame))
		size += int(unsafe.Sizeof(u.Input.NumBits))
		size += int(unsafe.Sizeof(u.Input.InputSize))
		size += int(u.Input.NumBits+7) / 8
		return size, nil
	}
	return 0, errors.New("ggthx UdpMsg PayloadSize: invalid packet type, could not find payload size")
}

// might just wanna make this a log function specifically, but this'll do for not
func (u UdpMsg) String() string {
	str := ""
	switch UdpMsgType(u.Header.HeaderType) {
	case SyncRequestMsg:
		str = fmt.Sprintf("sync-request (%d).\n", u.SyncRequest.RandomRequest)
	case SyncReplyMsg:
		str = fmt.Sprintf("sync-reply (%d).\n", u.SyncReply.RandomReply)
	case QualityReportMsg:
		str = "quality report.\n"
	case QualityReplyMsg:
		str = "quality reply.\n"
	case InputAckMsg:
		str = "input ack.\n"
	case KeepAliveMsg:
		str = "keep alive.\n"
	case InputMsg:
		str = fmt.Sprintf("game-compressed-input %d (+ %d bits).\n",
			u.Input.StartFrame, u.Input.NumBits)
	}
	return str
}
