package i18n

import "fmt"

// MessageKey 消息键常量
type MessageKey string

const (
	// 通用错误
	InvalidParameters MessageKey = "invalid_parameters"

	// 房间相关错误
	RoomAlreadyExists           MessageKey = "room_already_exists"
	ChannelHasActiveRoom        MessageKey = "channel_has_active_room"
	CreatorInAnotherCall        MessageKey = "creator_in_another_call"
	ParticipantInCall           MessageKey = "participant_in_call"
	RoomNotFound                MessageKey = "room_not_found"
	RoomNotActive               MessageKey = "room_not_active"
	RoomCreationFailed          MessageKey = "room_creation_failed"
	CreatorParticipantAddFailed MessageKey = "creator_participant_add_failed"
	InvitedParticipantAddFailed MessageKey = "invited_participant_add_failed"
	TokenGenerationFailed       MessageKey = "token_generation_failed"

	// 参与者相关错误
	ParticipantNotFound           MessageKey = "participant_not_found"
	ParticipantQueryFailed        MessageKey = "participant_query_failed"
	ParticipantJoinFailed         MessageKey = "participant_join_failed"
	ParticipantLeaveFailed        MessageKey = "participant_leave_failed"
	ParticipantStatusUpdateFailed MessageKey = "participant_status_update_failed"
	ParticipantListQueryFailed    MessageKey = "participant_list_query_failed"
	ParticipantNotInvited         MessageKey = "participant_not_invited"
)

// Translations 多语言翻译映射
var Translations = map[string]map[MessageKey]string{
	"zh-CN": {
		InvalidParameters:             "参数错误",
		RoomAlreadyExists:             "房间已存在: %s",
		ChannelHasActiveRoom:          "该渠道已存在正在通话的房间",
		CreatorInAnotherCall:          "创建者正在进行其他通话，无法创建房间",
		ParticipantInCall:             "参与者 %s 正在通话中，无法邀请",
		RoomNotFound:                  "房间不存在: %s",
		RoomNotActive:                 "房间已结束或已取消，无法加入",
		RoomCreationFailed:            "创建房间失败: %v",
		CreatorParticipantAddFailed:   "添加创建者参与者记录失败: %v",
		InvitedParticipantAddFailed:   "添加邀请参与者记录失败: %v",
		TokenGenerationFailed:         "生成 Token 失败: %v",
		ParticipantNotFound:           "参与者不存在: %s",
		ParticipantQueryFailed:        "查询参与者失败: %v",
		ParticipantJoinFailed:         "参与者加入房间失败: %v",
		ParticipantLeaveFailed:        "参与者离开房间失败: %v",
		ParticipantStatusUpdateFailed: "更新参与者状态失败: %v",
		ParticipantListQueryFailed:    "查询参与者列表失败: %v",
		ParticipantNotInvited:         "您未被邀请加入此房间",
	},
	"zh-TW": {
		InvalidParameters:             "參數錯誤",
		RoomAlreadyExists:             "房間已存在: %s",
		ChannelHasActiveRoom:          "該渠道已存在正在通話的房間",
		CreatorInAnotherCall:          "建立者正在進行其他通話，無法建立房間",
		ParticipantInCall:             "參與者 %s 正在通話中，無法邀請",
		RoomNotFound:                  "房間不存在: %s",
		RoomNotActive:                 "房間已結束或已取消，無法加入",
		RoomCreationFailed:            "建立房間失敗: %v",
		CreatorParticipantAddFailed:   "新增建立者參與者記錄失敗: %v",
		InvitedParticipantAddFailed:   "新增邀請參與者記錄失敗: %v",
		TokenGenerationFailed:         "生成 Token 失敗: %v",
		ParticipantNotFound:           "參與者不存在: %s",
		ParticipantQueryFailed:        "查詢參與者失敗: %v",
		ParticipantJoinFailed:         "參與者加入房間失敗: %v",
		ParticipantLeaveFailed:        "參與者離開房間失敗: %v",
		ParticipantStatusUpdateFailed: "更新參與者狀態失敗: %v",
		ParticipantListQueryFailed:    "查詢參與者列表失敗: %v",
		ParticipantNotInvited:         "您未被邀請加入此房間",
	},
	"en-US": {
		InvalidParameters:             "Invalid parameters",
		RoomAlreadyExists:             "Room already exists: %s",
		ChannelHasActiveRoom:          "An active room already exists for this channel",
		CreatorInAnotherCall:          "Creator is in another call, cannot create room",
		ParticipantInCall:             "Participant %s is in a call, cannot invite",
		RoomNotFound:                  "Room not found: %s",
		RoomNotActive:                 "Room has ended or been cancelled, cannot join",
		RoomCreationFailed:            "Failed to create room: %v",
		CreatorParticipantAddFailed:   "Failed to add creator participant record: %v",
		InvitedParticipantAddFailed:   "Failed to add invited participant record: %v",
		TokenGenerationFailed:         "Failed to generate token: %v",
		ParticipantNotFound:           "Participant not found: %s",
		ParticipantQueryFailed:        "Failed to query participant: %v",
		ParticipantJoinFailed:         "Failed to join room: %v",
		ParticipantLeaveFailed:        "Failed to leave room: %v",
		ParticipantStatusUpdateFailed: "Failed to update participant status: %v",
		ParticipantListQueryFailed:    "Failed to query participant list: %v",
		ParticipantNotInvited:         "You are not invited to join this room",
	},
	"fr-FR": {
		InvalidParameters:             "Paramètres invalides",
		RoomAlreadyExists:             "La salle existe déjà: %s",
		ChannelHasActiveRoom:          "Une salle active existe déjà pour ce canal",
		CreatorInAnotherCall:          "Le créateur est en appel, impossible de créer la salle",
		ParticipantInCall:             "Le participant %s est en appel, impossible d'inviter",
		RoomNotFound:                  "Salle non trouvée: %s",
		RoomNotActive:                 "La salle a pris fin ou a été annulée, impossible de rejoindre",
		RoomCreationFailed:            "Échec de la création de la salle: %v",
		CreatorParticipantAddFailed:   "Échec de l'ajout du participant créateur: %v",
		InvitedParticipantAddFailed:   "Échec de l'ajout du participant invité: %v",
		TokenGenerationFailed:         "Échec de la génération du jeton: %v",
		ParticipantNotFound:           "Participant non trouvé: %s",
		ParticipantQueryFailed:        "Échec de la requête du participant: %v",
		ParticipantJoinFailed:         "Échec de l'accès à la salle: %v",
		ParticipantLeaveFailed:        "Échec de la sortie de la salle: %v",
		ParticipantStatusUpdateFailed: "Échec de la mise à jour du statut du participant: %v",
		ParticipantListQueryFailed:    "Échec de la requête de la liste des participants: %v",
		ParticipantNotInvited:         "Vous n'êtes pas invité à rejoindre cette salle",
	},
	"ja-JP": {
		InvalidParameters:             "無効なパラメータ",
		RoomAlreadyExists:             "ルームは既に存在します: %s",
		ChannelHasActiveRoom:          "このチャネルには既にアクティブなルームが存在します",
		CreatorInAnotherCall:          "作成者は別の通話中です。ルームを作成できません",
		ParticipantInCall:             "参加者 %s は通話中です。招待できません",
		RoomNotFound:                  "ルームが見つかりません: %s",
		RoomNotActive:                 "ルームは終了またはキャンセルされています。参加できません",
		RoomCreationFailed:            "ルームの作成に失敗しました: %v",
		CreatorParticipantAddFailed:   "作成者参加者レコードの追加に失敗しました: %v",
		InvitedParticipantAddFailed:   "招待された参加者レコードの追加に失敗しました: %v",
		TokenGenerationFailed:         "トークンの生成に失敗しました: %v",
		ParticipantNotFound:           "参加者が見つかりません: %s",
		ParticipantQueryFailed:        "参加者のクエリに失敗しました: %v",
		ParticipantJoinFailed:         "ルームへの参加に失敗しました: %v",
		ParticipantLeaveFailed:        "ルームからの退出に失敗しました: %v",
		ParticipantStatusUpdateFailed: "参加者ステータスの更新に失敗しました: %v",
		ParticipantListQueryFailed:    "参加者リストのクエリに失敗しました: %v",
		ParticipantNotInvited:         "このルームへの招待を受けていません",
	},
}

// Translate 翻译消息
func Translate(lang string, key MessageKey, args ...interface{}) string {
	if !IsLanguageSupported(lang) {
		lang = DefaultLanguage
	}

	if translations, ok := Translations[lang]; ok {
		if message, ok := translations[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(message, args...)
			}
			return message
		}
	}

	// 如果翻译不存在，返回中文版本
	if translations, ok := Translations[DefaultLanguage]; ok {
		if message, ok := translations[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(message, args...)
			}
			return message
		}
	}

	return string(key)
}
