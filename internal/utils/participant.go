package utils

import (
	"tgo-rtc-server/internal/models"
)

// ParticipantDeduplicator 参与者去重工具
type ParticipantDeduplicator struct{}

// NewParticipantDeduplicator 创建参与者去重工具
func NewParticipantDeduplicator() *ParticipantDeduplicator {
	return &ParticipantDeduplicator{}
}

// DeduplicateUIDs 对 UID 列表进行去重
// 返回去重后的 UID 列表，保持原始顺序
func (pd *ParticipantDeduplicator) DeduplicateUIDs(uids []string) []string {
	if len(uids) == 0 {
		return uids
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(uids))

	for _, uid := range uids {
		if uid != "" && !seen[uid] {
			seen[uid] = true
			result = append(result, uid)
		}
	}

	return result
}

// DeduplicateParticipants 对参与者列表进行去重
// 按 UID 去重，保留第一个出现的记录
func (pd *ParticipantDeduplicator) DeduplicateParticipants(participants []models.Participant) []models.Participant {
	if len(participants) == 0 {
		return participants
	}

	seen := make(map[string]bool)
	result := make([]models.Participant, 0, len(participants))

	for _, p := range participants {
		if p.UID != "" && !seen[p.UID] {
			seen[p.UID] = true
			result = append(result, p)
		}
	}

	return result
}

// RemoveDuplicateUIDs 从 UID 列表中移除指定的 UID
// 用于从邀请列表中移除创建者或已存在的参与者
func (pd *ParticipantDeduplicator) RemoveDuplicateUIDs(uids []string, excludeUIDs ...string) []string {
	if len(uids) == 0 {
		return uids
	}

	// 构建排除集合
	excludeSet := make(map[string]bool)
	for _, uid := range excludeUIDs {
		if uid != "" {
			excludeSet[uid] = true
		}
	}

	result := make([]string, 0, len(uids))
	seen := make(map[string]bool)

	for _, uid := range uids {
		if uid != "" && !seen[uid] && !excludeSet[uid] {
			seen[uid] = true
			result = append(result, uid)
		}
	}

	return result
}

// MergeAndDeduplicateUIDs 合并多个 UID 列表并去重
// 按顺序合并列表，保持原始顺序，去除重复
func (pd *ParticipantDeduplicator) MergeAndDeduplicateUIDs(uidLists ...[]string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)

	for _, uids := range uidLists {
		for _, uid := range uids {
			if uid != "" && !seen[uid] {
				seen[uid] = true
				result = append(result, uid)
			}
		}
	}

	return result
}

// ContainsUID 检查 UID 列表中是否包含指定的 UID
func (pd *ParticipantDeduplicator) ContainsUID(uids []string, uid string) bool {
	for _, u := range uids {
		if u == uid {
			return true
		}
	}
	return false
}

// FilterUIDs 根据条件过滤 UID 列表
// predicate 返回 true 表示保留该 UID
func (pd *ParticipantDeduplicator) FilterUIDs(uids []string, predicate func(uid string) bool) []string {
	result := make([]string, 0, len(uids))

	for _, uid := range uids {
		if uid != "" && predicate(uid) {
			result = append(result, uid)
		}
	}

	return result
}

