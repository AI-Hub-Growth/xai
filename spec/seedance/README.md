# Seedance（火山方舟视频生成）

本包定义 **Seedance / 豆包视频生成** 在 xai 中的规格层：URI scheme、`GenVideo` 操作、参数名与异步结果类型。HTTP 接入由 [`provider/volc`](provider/volc/README.md) 实现。

官方文档（设计与联调时请以文档为准）：

- [创建视频生成任务 API](https://www.volcengine.com/docs/82379/1520757?lang=zh)（Seedance 2.0 等）
- [查询视频生成任务 API](https://www.volcengine.com/docs/82379/1521309?lang=zh)
- [Base URL 及鉴权](https://www.volcengine.com/docs/82379/1298459?lang=zh)

## 职责划分

| 组件 | 说明 |
|------|------|
| `seedance.Scheme` (`seedance://`) | 通过 `seedance.Register(svc)` 注册到 `xai.New` |
| `Service` + `Backend` | 仅支持 `FeatureOperation`；`GenVideo` 提交任务并轮询 |
| `Params` | 仅暴露与当前实现对齐的一组字段；`content` 由 `text`/`prompt` 与参考 URL 合成 |
| `schema.GenVideoFields` | `InputSchema` 字段列表，与 `params.go` 常量一致 |

## 与 Ark 请求体的对应关系

`model` 由操作层传入（**非** `Params`）。`Params` 与 Ark 的对应关系如下（其余官方根级字段如 `resolution`、`callback_url`、`tools` 等**不在**本包参数中，需自行扩展实现或在提示词中使用官方文档说明的 `--` 弱校验方式）。

### `content` 合成规则

| xai 参数名（常量见 `params.go`） | Ark 行为 |
|----------------------------------|----------|
| `text` 或 `prompt` | 必选；在 `content` 中写入 `{ "type":"text", "text":"..." }` |
| `reference_image_urls` | 多条 `{ "type":"image_url", "image_url":{"url":"..."}, "role":"reference_image" }` |
| `reference_video_urls` | `type`=`video_url`，`role`=`reference_video` |
| `reference_audio_urls` | `type`=`audio_url`，`role`=`reference_audio` |

### 根级字段（本包写入）

| 参数名 | 说明 |
|--------|------|
| `duration` | 整数秒；**不传**时请求体不含该字段，方舟默认 **5** 秒（见官方文档）。`0` 非法。[`ValidateVideoDuration`](duration.go) 按 Model ID 校验 **1.0 / 1.5 / 2.0** 文档区间与是否允许 `-1`。 |
| `ratio` | 如 `16:9`、`adaptive` |
| `generate_audio` | bool |
| `watermark` | bool |
| `auto_create_assets` | bool；由支持该能力的 provider 透传，用于任务内自动创建和清理临时素材 |

### `duration` 官方时长范围（客户端校验摘要）

| 模型线（文档） | 合法 `duration`（秒） |
|----------------|----------------------|
| Seedance 1.0 pro / pro fast / lite | **`[2, 12]`**；**不支持 `-1`** |
| Seedance 1.5 pro | **`[4, 12]`** 或 **`-1`**（智能时长） |
| Seedance 2.0 / 2.0 fast | **`[4, 15]`** 或 **`-1`** |

无法从 model id 识别的 Endpoint：仅拒绝 `0`，其余交由接口校验。

更多 HTTP 与调试说明见 [`provider/volc/README.md`](provider/volc/README.md)。

## 任务状态

查询接口返回的 `status` 在文档中多为排队 / 运行中 / 成功 / 失败等英文枚举；实现侧会做大小写无关归一化，并在 `succeeded`（及兼容的 `completed` 等）时解析 `video_url`（含嵌套在 `content`、`data.output` 等常见形态）。细节以[查询视频生成任务 API](https://www.volcengine.com/docs/82379/1521309?lang=zh)为准。

异步任务创建后 `OperationResponse.Done()` 为 false，需用 `xai.Wait` 轮询；`AsyncOperationResponse` 默认每次 `Retry` 前 `Sleep` 约 **2 秒**。调用 `xai.Wait(ctx, svc, resp, progress)` 时建议传入非 nil 的 `progress`，在长时间生成时打印 `TaskID()` 等，避免终端长时间无输出（见 `examples/videorouter`、`examples/seedance`）。

## 代码入口

```go
import (
    "github.com/goplus/xai/spec/seedance"
    "github.com/goplus/xai/spec/seedance/provider/volc"
)

volc.Register(os.Getenv("ARK_API_KEY"))
// xai.New(ctx, "seedance://")
```

完整示例见仓库 [`examples/seedance`](../../examples/seedance)。  
按 **模型 id 自动选 Volc Ark 或七牛 Kling** 的 GenVideo 示例：[`examples/videorouter`](../../examples/videorouter)（实现见 [`examples/shared/video_by_model.go`](../../examples/shared/video_by_model.go)）。
