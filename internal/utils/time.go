package utils

import "time"

// TimeFormatter 时间格式化工具
type TimeFormatter struct{}

// NewTimeFormatter 创建时间格式化工具实例
func NewTimeFormatter() *TimeFormatter {
	return &TimeFormatter{}
}

// FormatDateTime 格式化时间为 yyyy-mm-dd hh:mm:ss 格式
func (tf *TimeFormatter) FormatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// FormatDate 格式化时间为 yyyy-mm-dd 格式
func (tf *TimeFormatter) FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatTime 格式化时间为 hh:mm:ss 格式
func (tf *TimeFormatter) FormatTime(t time.Time) string {
	return t.Format("15:04:05")
}

// FormatISO8601 格式化时间为 ISO8601 格式
func (tf *TimeFormatter) FormatISO8601(t time.Time) string {
	return t.Format(time.RFC3339)
}

