# SDK Watcher 集成

SDK 服务暴露了 watcher 集成，可以呈现细粒度的认证更新而无需强制完整重载。本文档说明队列契约、服务如何消费更新，以及如何处理高频变更突发。

## 更新队列契约

- `watcher.AuthUpdate` 表示一次凭据变更。`Action` 可以是 `add`、`modify` 或 `delete`，`ID` 携带凭据标识。对于 `add`/`modify`，`Auth` 负载包含一个完整填充的凭据克隆；`delete` 可以省略 `Auth`。
- `WatcherWrapper.SetAuthUpdateQueue(chan<- watcher.AuthUpdate)` 将 SDK 服务产生的队列接入 watcher。队列必须在 watcher 启动前创建。
- 服务通过 `ensureAuthUpdateQueue` 构建队列，使用缓冲通道与专用消费 goroutine（`consumeAuthUpdates`）。消费者通过循环处理后面积压数据再重新进入 select 循环来排空突发。

## Watcher 行为

- `internal/watcher/watcher.go` 维护认证状态的影子快照（`currentAuths`）。每次文件系统或配置事件都会触发重新计算，并与之前的快照做差异比较，产生最小化的 `AuthUpdate` 条目，反映新增、修改与删除。
- 更新按凭据标识合并。如果在分发前发生多次变更（例如先写入后删除），只会向下游发送最终动作。
- watcher 运行内部分发循环，将待处理更新缓冲在内存中并异步转发到队列。生产者从不阻塞在通道容量上；它们只将更新入队到内存缓冲并通知分发器。watcher 停止时取消分发，保证 goroutine 干净退出。

## 高频变更处理

- 分发循环与服务消费者独立运行，即使同时到达大量更新，文件系统 watcher 也不会阻塞。
- 背压在两处被吸收：
  - 分发缓冲在消费者追上之前合并同一凭据的重复更新。
  - 服务通道容量与消费者排空循环确保多个突发无需振荡即可被处理。
- 如果队列长时间饱和，更新会继续合并，因此最终会应用最新状态，而不会重放冗余的中间状态。

## 使用清单

1. 实例化 SDK 服务（builder 或手动构造）。
2. 在启动 watcher 前调用 `ensureAuthUpdateQueue` 分配共享通道。
3. 创建 `WatcherWrapper` 后，用服务队列调用 `SetAuthUpdateQueue`，然后启动 watcher。
4. 提供处理配置更新的重载回调；认证增量会通过队列到达，并由服务通过 `handleAuthUpdate` 自动应用。

遵循此流程可保持认证变更响应迅速，同时避免每次编辑都触发完整重载。
