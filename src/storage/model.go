package storage

import "time"

// User 用户表（对应文章的用户状态管理）
type User struct {
	ID           uint      `gorm:"primarykey"`
	UserName     string    `gorm:"uniqueIndex;size:50;not null"`
	PasswordHash string    `gorm:"size:100;not null"` // bcrypt加密
	NickName     string    `gorm:"size:50"`
	AvatarURL    string    `gorm:"size:200"`
	Status       int8      `gorm:"default:0;comment:'0-离线 1-在线'"` // 对应用户在线状态
	CreateTime   time.Time `gorm:"autoCreateTime"`
	UpdateTime   time.Time `gorm:"autoUpdateTime"`
}

// Message 消息表（对应文章的消息存储，按会话分区）
type Message struct {
	ID          uint      `gorm:"primarykey"`
	ClientMsgID string    `gorm:"size:64;not null;uniqueIndex:uk_from_client,priority:2"`       // 客户端消息ID（幂等性）
	ServerMsgID string    `gorm:"uniqueIndex;size:64;not null"`                                 // 服务端消息ID（全局唯一）
	Seq         uint64    `gorm:"index;not null"`                                               // 消息序号（用于排序和同步）
	ChatType    int8      `gorm:"index;not null;comment:'1-单聊 2-群聊'"`                           // 会话类型
	ChatID      string    `gorm:"index;size:64;not null"`                                       // 会话ID
	SendID      string    `gorm:"index;size:64;not null;uniqueIndex:uk_from_client,priority:1"` // 发送者ID
	RecvID      string    `gorm:"index;size:64;not null"`                                       // 接收者ID
	ContentType int8      `gorm:"not null;comment:'1-文本 2-图片...'"`                              // 内容类型
	Content     string    `gorm:"type:text;not null"`                                           // 消息内容
	Status      int8      `gorm:"default:0;comment:'0-发送中 1-成功 2-失败'"`
	CreateTime  time.Time `gorm:"autoCreateTime;index"` // 创建时间（用于Timeline查询）
}

// MessageRecipient 投递状态表（用于离线重试、送达/已读回执）
type MessageRecipient struct {
	ID          uint       `gorm:"primarykey"`
	ServerMsgID string     `gorm:"size:64;not null;uniqueIndex:uk_msg_to,priority:1;index"`
	ToUser      string     `gorm:"size:64;not null;uniqueIndex:uk_msg_to,priority:2;index"`
	Status      int8       `gorm:"index;default:0;comment:'0-pending 1-delivered 2-read'"`
	RetryCount  int        `gorm:"default:0"`
	NextRetryAt *time.Time `gorm:"index"`
	LastError   string     `gorm:"size:255"`
	LastSendAt  *time.Time
	AckedAt     *time.Time
	ReadAt      *time.Time
	CreateTime  time.Time `gorm:"autoCreateTime;index"`
	UpdateTime  time.Time `gorm:"autoUpdateTime"`
}

// Room 房间表（对应群聊功能）
type Room struct {
	ID         uint      `gorm:"primarykey"`
	RoomID     string    `gorm:"uniqueIndex;size:64;not null"`
	Name       string    `gorm:"size:100;not null"`
	OwnerID    string    `gorm:"index;size:64;not null"`
	CreateTime time.Time `gorm:"autoCreateTime"`
}

// GroupMember 群成员表
type GroupMember struct {
	ID       uint      `gorm:"primarykey"`
	RoomID   string    `gorm:"index;size:64;not null"`
	UserID   string    `gorm:"index;size:64;not null"`
	Role     int8      `gorm:"default:0;comment:'0-普通成员 1-管理员 2-群主'"`
	JoinTime time.Time `gorm:"autoCreateTime"`
}
