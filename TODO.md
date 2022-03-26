### TODO
- 支持消息压缩
- 支持多租户命名空间
- 消息防篡改（CRC校验）
- 发送消息到主题的权限及建立连接时登录


producer -> topic -> broker -> storage -> notify -> consumer -> ack -> broker -> record consumer offset

components:
producer
consumer(support consume group)
broker(support cluster)
storage(support cluster)

metadata:
topic info
broker info
storage info
consumer subscription relationship
consumer consume offset



