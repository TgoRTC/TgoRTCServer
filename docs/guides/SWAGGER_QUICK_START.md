# Swagger 文档更新 - 快速参考

## 🚀 最快的更新方式（推荐）

### 一键更新
```bash
./update-swagger-docs.sh
```

这个脚本会自动：
1. ✅ 验证 YAML 语法
2. ✅ 转换 YAML 为 JSON
3. ✅ 验证 JSON 格式
4. ✅ 更新 docs.go
5. ✅ 构建项目

---

## 📝 手动更新步骤

### 步骤 1: 编辑 API 文档
```bash
# 编辑 swagger.yaml
vim docs/swagger.yaml
```

### 步骤 2: 转换为 JSON
```bash
# 方式 A: 使用 yq（推荐）
yq eval -o=json docs/swagger.yaml > docs/swagger.json

# 方式 B: 使用 Python
python3 << 'EOF'
import yaml, json
with open('docs/swagger.yaml', 'r', encoding='utf-8') as f:
    yaml_data = yaml.safe_load(f)
with open('docs/swagger.json', 'w', encoding='utf-8') as f:
    json.dump(yaml_data, f, ensure_ascii=False, indent=2)
EOF
```

### 步骤 3: 更新 docs.go
```bash
# 复制 swagger.json 的内容到 docs/docs.go 的 docTemplate 中
# 或使用脚本自动更新
./update-swagger-docs.sh
```

### 步骤 4: 构建并测试
```bash
go build -o tgo-rtc-server
./tgo-rtc-server &
sleep 3

# 验证
curl -s http://localhost:8080/swagger/index.html | head -20

# 停止
pkill -f tgo-rtc-server
```

---

## 📋 常见操作

### 添加新 API 端点

1. **编辑 `docs/swagger.yaml`**
```yaml
paths:
  /api/v1/new-endpoint:
    post:
      tags:
        - 功能分类
      summary: 端点描述
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                field1:
                  type: string
      responses:
        '200':
          description: 成功
```

2. **运行更新脚本**
```bash
./update-swagger-docs.sh
```

---

### 修改现有端点

1. **在 `docs/swagger.yaml` 中找到并修改**
2. **运行更新脚本**
```bash
./update-swagger-docs.sh
```

---

### 添加新的数据模型

1. **在 `docs/swagger.yaml` 的 `components/schemas` 中添加**
```yaml
components:
  schemas:
    NewModel:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
```

2. **在端点中引用**
```yaml
responses:
  '200':
    description: 成功
    content:
      application/json:
        schema:
          $ref: '#/components/schemas/NewModel'
```

3. **运行更新脚本**
```bash
./update-swagger-docs.sh
```

---

## ✅ 验证清单

更新后检查以下项目：

- [ ] `docs/swagger.yaml` 语法正确
- [ ] `docs/swagger.json` 格式有效
- [ ] `docs/docs.go` 已更新
- [ ] 项目构建成功：`go build -o tgo-rtc-server`
- [ ] 服务启动正常：`./tgo-rtc-server`
- [ ] Swagger UI 可访问：`http://localhost:8080/swagger/index.html`
- [ ] 所有 API 端点都显示了
- [ ] 参数和响应定义正确

---

## 🔍 故障排除

### 问题：脚本执行失败

**解决方案：**
```bash
# 检查依赖
which yq      # 应该输出 yq 的路径
which python3 # 应该输出 python3 的路径

# 安装缺失的工具
brew install yq  # macOS
apt-get install yq  # Linux
```

### 问题：JSON 格式错误

**解决方案：**
```bash
# 验证 JSON
python3 -m json.tool docs/swagger.json

# 或使用在线工具
# https://jsonlint.com/
```

### 问题：Swagger UI 显示 404

**解决方案：**
```bash
# 检查服务是否运行
ps aux | grep tgo-rtc-server

# 重新构建
go build -o tgo-rtc-server

# 重启服务
pkill -f tgo-rtc-server
./tgo-rtc-server
```

---

## 📚 文件说明

| 文件 | 说明 |
|------|------|
| `docs/swagger.yaml` | OpenAPI 3.0 YAML 格式（主要编辑文件） |
| `docs/swagger.json` | OpenAPI 3.0 JSON 格式（自动生成） |
| `docs/docs.go` | swaggo 库使用的定义（自动生成） |
| `update-swagger-docs.sh` | 自动更新脚本 |
| `SWAGGER_UPDATE_GUIDE.md` | 详细更新指南 |

---

## 🎯 工作流程

```
编辑 swagger.yaml
        ↓
运行 ./update-swagger-docs.sh
        ↓
自动转换为 JSON
        ↓
自动更新 docs.go
        ↓
自动构建项目
        ↓
测试 Swagger UI
        ↓
提交代码
```

---

## 💡 提示

- 始终先编辑 `swagger.yaml`
- 使用脚本自动化更新过程
- 在提交前验证所有文件
- 保持三个文件同步
- 定期检查 Swagger UI 是否正确显示

---

## 🔗 相关资源

- [OpenAPI 3.0 规范](https://spec.openapis.org/oas/v3.0.3)
- [Swagger 编辑器](https://editor.swagger.io/)
- [详细更新指南](./SWAGGER_UPDATE_GUIDE.md)


