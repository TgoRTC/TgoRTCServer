package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"tgo-rtc-server/internal/utils"

	"go.uber.org/zap"
)

// MigrationScript 迁移脚本结构
type MigrationScript struct {
	Version string
	Name    string
	SQL     string
}

// LoadMigrations 从 migrations 目录加载所有迁移脚本
func LoadMigrations() ([]MigrationScript, error) {
	// 获取 migrations 目录路径
	migrationsDir := "migrations"

	// 如果目录不存在，尝试从上级目录查找
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// 尝试从项目根目录查找
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("获取工作目录失败: %w", err)
		}
		migrationsDir = filepath.Join(wd, "migrations")
	}

	// 读取 migrations 目录
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("读取 migrations 目录失败: %w", err)
	}

	var scripts []MigrationScript

	// 遍历所有 .sql 文件
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		filePath := filepath.Join(migrationsDir, entry.Name())

		// 读取文件内容
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取文件 %s 失败: %w", filePath, err)
		}

		// 解析文件名获取版本号和名称
		version, name := parseMigrationFileName(entry.Name())
		if version == "" {
			continue
		}

		// 提取 SQL 语句（去除注释和空行）
		sql := extractSQL(string(content))
		if sql == "" {
			logger := utils.GetLogger()
			logger.Warn("⚠️  警告: 文件中没有找到有效的 SQL 语句",
				zap.String("file", entry.Name()),
			)
			continue
		}

		scripts = append(scripts, MigrationScript{
			Version: version,
			Name:    name,
			SQL:     sql,
		})
	}

	// 按版本号排序
	sort.Slice(scripts, func(i, j int) bool {
		return scripts[i].Version < scripts[j].Version
	})

	if len(scripts) == 0 {
		return nil, fmt.Errorf("在 %s 目录中没有找到任何迁移脚本", migrationsDir)
	}

	logger := utils.GetLogger()
	logger.Info("✅ 成功加载迁移脚本",
		zap.Int("count", len(scripts)),
	)
	return scripts, nil
}

// parseMigrationFileName 解析迁移文件名
// 支持两种格式:
// 1. 日期+序号格式: 20251027-01.sql (推荐)
// 2. 版本号+描述格式: 001_create_rtc_room_table.sql (兼容)
func parseMigrationFileName(filename string) (version, name string) {
	// 移除 .sql 扩展名
	nameWithoutExt := strings.TrimSuffix(filename, ".sql")

	// 尝试匹配日期+序号格式: 20251027-01
	dateSeqRe := regexp.MustCompile(`^(\d{8})-(\d{2})$`)
	if matches := dateSeqRe.FindStringSubmatch(nameWithoutExt); len(matches) == 3 {
		date := matches[1] // 20251027
		seq := matches[2]  // 01
		version = date + "-" + seq
		name = "Migration " + date + "-" + seq
		return version, name
	}

	// 尝试匹配版本号+描述格式: 001_create_rtc_room_table
	versionDescRe := regexp.MustCompile(`^(\d+)_(.+)$`)
	if matches := versionDescRe.FindStringSubmatch(nameWithoutExt); len(matches) == 3 {
		version = matches[1]
		description := matches[2]

		// 将下划线替换为空格
		description = strings.ReplaceAll(description, "_", " ")

		return version, description
	}

	return "", ""
}

// extractSQL 从文件内容中提取 SQL 语句
func extractSQL(content string) string {
	lines := strings.Split(content, "\n")
	var sqlLines []string

	for _, line := range lines {
		// 移除注释行
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}

		// 移除空行
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		sqlLines = append(sqlLines, line)
	}

	sql := strings.Join(sqlLines, "\n")
	sql = strings.TrimSpace(sql)

	return sql
}

// RunMigrations 运行所有迁移
func RunMigrations(mm *MigrationManager) error {
	// 加载迁移脚本
	scripts, err := LoadMigrations()
	if err != nil {
		return fmt.Errorf("加载迁移脚本失败: %w", err)
	}

	// 执行每个迁移脚本
	for _, script := range scripts {
		if err := mm.ExecuteMigration(script.Version, script.Name, script.SQL); err != nil {
			return err
		}
	}
	return nil
}
