# E2E 本地测试指南

## 前提条件

- MySQL 已启动（本地或 Docker）
- Redis 已启动（本地或 Docker）
- `.env` 文件已配置（DB_HOST=127.0.0.1）

## 快速开始

### 1. 运行测试

```bash
make e2e-local
```

### 2. 查看结果

测试结果保存在 `test-output/e2e_summary.txt`

## 测试内容

测试会自动调用以下 5 个 API 接口：

1. **POST /api/v1/rooms** - 创建房间
2. **POST /api/v1/rooms/{room_id}/invite** - 邀请参与者
3. **POST /api/v1/rooms/{room_id}/join** - 加入房间
4. **POST /api/v1/participants/calling** - 查询正在通话的成员
5. **POST /api/v1/rooms/{room_id}/leave** - 离开房间

所有接口都期望返回 **HTTP 200** 状态码。

## 成功输出示例

```
[E2E] 检测到服务已就绪，跳过启动
[E2E] 服务健康检查通过
[E2E] 清理旧的测试数据...
[E2E] create_room: HTTP 200
[E2E] room_id=7f40bbb77d37410e94f86f90437997b7
[E2E] invite: HTTP 200
[E2E] join: HTTP 200
[E2E] calling: HTTP 200
[E2E] leave: HTTP 200
[E2E] [PASS] create_room 期望 200 得到 200
[E2E] [PASS] invite 期望 200 得到 200
[E2E] [PASS] join 期望 200 得到 200
[E2E] [PASS] calling 期望 200 得到 200
[E2E] [PASS] leave 期望 200 得到 200
[E2E] [PASS] create_room 响应包含字段 room_id
[E2E] [PASS] create_room 响应包含字段 token
[E2E] [PASS] create_room 响应包含字段 url
[E2E] [PASS] join 响应包含字段 room_id
[E2E] [PASS] join 响应包含字段 token
[E2E] [PASS] join 响应包含字段 url
[E2E] [PASS] calling 响应是数组格式
[E2E] 测试完成。详细响应见 /path/to/test-output/*.json，日志：/path/to/test-output/server.log
```

## 常见问题

**1. 数据库连接失败（Access denied for user 'root'@'192.168.65.1'）**

如果 MySQL 运行在 Docker 中，需要授权：
```bash
mysql -u root -e "CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY ''; GRANT ALL PRIVILEGES ON *.* TO 'root'@'%'; FLUSH PRIVILEGES;"
```

**2. 端口被占用**
```bash
lsof -i :8080
kill -9 <PID>
```

**3. 查看测试数据（Navicat）**
```sql
SELECT * FROM rtc_room WHERE creator LIKE 'test_%' ORDER BY created_at DESC LIMIT 10;
SELECT * FROM rtc_participant WHERE uid LIKE 'test_%' ORDER BY created_at DESC LIMIT 20;
```

