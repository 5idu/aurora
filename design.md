## 通信设计
### 序列化和反序列化选择
[protocol-buffers](https://developers.google.com/protocol-buffers)

### 通信协议
自定义通信协议，全双工通信

### 消息定义
<!-- [CLIENT_VERSION] | [PROTOCAL_VERSION] -->
[TOTAL_SIZE] | [CMD_SIZE] | [CMD] | [METADATA_SIZE] | [METADATA] | [PAYLOAD]
- TOTAL_SIZE：消息总大小
- CMD_SIZE：命令大小
- CMD：命令 // pb 序列化后的命令
- METADATA_SIZE：元数据大小
- METADATA：元数据 // pb 序列化后的元数据
- PAYLOAD：消息体 // 原始数据二进制数据

### 两种类型的消息
- Simple commands: that do not carry a message payload.
    - total size: 4 bytes，数据包的总长度，等于除了 total size 以外的其他字段的总长度
    - command size: 4 bytes，等于 protobuf-serialized command 的大小
    - message: protobuf-serialized command
- Payload commands: that bear a payload that is used when publishing or delivering messages. 
    - totalSize: 4 bytes
    - command size: 4 bytes
    - message: protobuf-serialized command
    - payload size: 4 bytes
    - magicNumber: 2 bytes, message version number
    - checksum: 4 bytes, CRC32 checksum of the (metadataSize + metadata + payload)
    - metadataSize: 4 bytes, size of the metadata
    - metadata: message metadata
    - payload: message payload

- message metadata
消息元数据(metadata)将与有效负载(payload)一起存储。元数据由生产者创建并原封不动地传递给消费者。
    - producer_name
    - sequence_id: The sequence ID of the message, assigned by producer
    - publish_time
    - properties：附加的 K/V 属性
    - compression：有效载荷(payload)的压缩方式
    - uncompressed_size: 当使用了压缩时，该字段指示未压缩(payload)的消息大小
    - num_messages_in_batch: 如果是批量消息，该字段指示批量消息中的消息数量
