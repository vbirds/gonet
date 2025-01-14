package chat

import (
	"gonet/actor"
	"gonet/base"
	"gonet/common/cluster"
	"gonet/rpc"
	"gonet/server/gm"
	"gonet/server/message"
	"time"

	"golang.org/x/net/context"
)

const (
	CHAT_PENDING_TIME_NORAML    = 1
	CHAT_PENDING_TIME_PRIVATE   = 1
	CHAT_PENDING_TIME_WORLDPLUS = 1
)

type (
	ChatMessage struct {
		Sender      int64
		Recver      int64
		MessageType int8
		Message     string
		SenderName  string
		RecverName  string
	}

	stPlayerChatRecord struct {
		nLastTime    int64
		nPendingTime int64
	}

	player struct {
		playerId      int64
		playerName    string
		gateClusterId uint32
	}

	ChatMgr struct {
		actor.Actor
		cluster.Stub
		m_channelManager ChannelMgr
		m_playerChatMap  map[int64]*stPlayerChatRecord
	}

	IChatMgr interface {
		actor.IActor

		SendMessageTo(msg *ChatMessage, playerId int64)
		SendMessageToAll(msg *ChatMessage)
		GetChannelManager() *ChannelMgr
		//setPlayerChatLastTime(int64, int8, int64)
		//getPlayerChatLastTime(int64, int8) int64
		//getPlayerChatPendingTime(int64, int8) int64
	}
)

var (
	MGR ChatMgr
)

func (this *ChatMgr) Init() {
	this.Actor.Init()
	this.m_playerChatMap = make(map[int64]*stPlayerChatRecord)
	this.m_channelManager.Init()
	actor.MGR.RegisterActor(this)
	this.Stub.InitStub(rpc.STUB_ChatMgr)
	this.Actor.Start()
}

func (this *ChatMgr) Stub_On_Register(ctx context.Context, Id int64) {
	//这里可以是加载db数据
	base.LOG.Println("Stub Chat register sucess")
}

func (this *ChatMgr) Stub_On_UnRegister(ctx context.Context, Id int64) {
	//lease一致性这里要清理缓存数据了
	base.LOG.Println("Stub Chat unregister sucess")
}

func (this *ChatMgr) GetChannelManager() *ChannelMgr {
	return &this.m_channelManager
}

func (this *ChatMgr) SendMessageTo(msg *ChatMessage, playerId int64) {
	pPlayer := this.m_channelManager.getChannel(g_wordChannelId).GetPlayer(playerId)
	if pPlayer != nil {
		SendMessage(msg, pPlayer)
	}
}

func SendMessage(msg *ChatMessage, player *player) {
	gm.SendToClient(rpc.RpcHead{ClusterId: player.gateClusterId}, &message.ChatMessageResponse{
		PacketHead:  message.BuildPacketHead(player.playerId, rpc.SERVICE_GATE),
		Sender:      msg.Sender,
		SenderName:  msg.SenderName,
		Recver:      msg.Recver,
		RecverName:  msg.RecverName,
		MessageType: int32(msg.MessageType),
		Message:     msg.Message,
	})
}

func (this *ChatMgr) SendMessageToAll(msg *ChatMessage) {
	cluster.MGR.SendMsg(rpc.RpcHead{DestServerType: rpc.SERVICE_GATE, SendType: rpc.SEND_BOARD_CAST},
		"Chat_SendMessageAll", msg)
}

func (this *ChatMgr) setPlayerChatLastTime(playerid int64, cMessageType int8, nTime int64) {
	v := int64(playerid)
	v = (v << 8) | int64(cMessageType)

	this.m_playerChatMap[v] = &stPlayerChatRecord{}
	this.m_playerChatMap[v].nLastTime = nTime
}

func (this *ChatMgr) getPlayerChatLastTime(playerid int64, cMessageType int8) int64 {
	v := int64(playerid)
	v = (v << 8) | int64(cMessageType)

	pData, exist := this.m_playerChatMap[v]
	if exist {
		return pData.nLastTime
	}
	return 0
}

func (this *ChatMgr) getPlayerChatPendingTime(playerid int64, cMessageType int8) int64 {
	v := int64(playerid)
	v = (v << 8) | int64(cMessageType)

	pData, exist := this.m_playerChatMap[v]
	if !exist {
		return 0
	}

	if pData.nPendingTime == 0 {
		switch cMessageType {
		case int8(message.CHAT_MSG_TYPE_PRIVATE):
			pData.nPendingTime = CHAT_PENDING_TIME_PRIVATE
		case int8(message.CHAT_MSG_TYPE_WORLD):
			pData.nPendingTime = CHAT_PENDING_TIME_WORLDPLUS
		default:
			pData.nPendingTime = CHAT_PENDING_TIME_NORAML
		}
	}

	return pData.nPendingTime
}

//聊天信息
func (this *ChatMgr) ChatMessageRequest(ctx context.Context, packet *message.ChatMessageRequest) {
	playerId := packet.GetSender()

	msg := &ChatMessage{}
	msg.Sender = playerId
	msg.SenderName = gm.SIMPLEMGR.GetPlayerName(msg.Sender)
	msg.Message = packet.GetMessage()
	msg.Recver = packet.GetRecver()
	msg.MessageType = int8(packet.GetMessageType())
	msg.RecverName = gm.SIMPLEMGR.GetPlayerName(msg.Recver)
	//替换屏蔽字库
	//data.ReplaceBanWord(msg.Message, "*")

	// 检查发送时间间隔
	nPendingTime := this.getPlayerChatPendingTime(playerId, msg.MessageType)
	nLastTime := this.getPlayerChatLastTime(playerId, msg.MessageType)
	nCurTime := time.Now().Unix()

	if nCurTime-nLastTime < nPendingTime {
		return
	}

	this.setPlayerChatLastTime(playerId, msg.MessageType, nCurTime)
	//writelog

	channelId := this.GetChannelManager().GetChannelIdByType(playerId, msg.MessageType)

	if msg.MessageType == int8(message.CHAT_MSG_TYPE_PRIVATE) && msg.Recver != msg.Sender { // 不能给自己发点对点消息
		this.SendMessageTo(msg, msg.Recver)
	} else if msg.MessageType == int8(message.CHAT_MSG_TYPE_WORLD) {
		//this.SendMessageToAll(msg)
		this.m_channelManager.SendMessageToChannel(msg, channelId)
	} else {
		if channelId == 0 {
			return
		}

		this.m_channelManager.SendMessageToChannel(msg, channelId)
	}
}

//注册频道
func (this *ChatMgr) RegisterChannel(ctx context.Context, messageType int8, channelId int64) {
	this.GetChannelManager().RegisterChannel(messageType, "", channelId)

	if 0 == channelId {
		return
	}

	if messageType == int8(message.CHAT_MSG_TYPE_ORG) {
	}
}

//销毁频道
func (this *ChatMgr) UnRegisterChannel(ctx context.Context, channelId int64) {
	this.GetChannelManager().UnregisterChannel(channelId)
}

//添加玩家到频道
func (this *ChatMgr) AddPlayerToChannel(ctx context.Context, playerId int64, channelId int64, playerName string, gateClusterId uint32) {
	this.GetChannelManager().AddPlayer(playerId, channelId, playerName, gateClusterId)
}

//删除玩家到频道
func (this *ChatMgr) RemovePlayerToChannel(ctx context.Context, playerId int64, channelId int64) {
	this.GetChannelManager().RemovePlayer(playerId, channelId)
}