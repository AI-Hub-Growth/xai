# Qiniu Provider（七牛 Qnagic）

本包实现 `seedance.Backend`，通过七牛 / Qnagic 的 `contents/generations/tasks` 接口调用 Seedance 视频生成。

## API

- 创建任务：`POST /v3/contents/generations/tasks`
- 查询任务：`GET /v3/contents/generations/tasks/{task_id}`
- 鉴权：`Authorization: Bearer <QINIU_API_KEY>`
- 默认 Base URL：`https://api.qnaigc.com`

参考文档：

- [创建任务](https://apidocs.qnaigc.com/439818050e0)
- [查询任务](https://apidocs.qnaigc.com/439818051e0)

## 本包映射

`seedance.Params` 到 Qiniu 请求体的映射如下：

- `text` / `prompt` -> `content[]` 的首个 `{type:"text"}`
- provider 私有 `resolution` -> 根级 `resolution`
- `ratio` -> 根级 `ratio`
- `duration` -> 根级 `duration`
- `generate_audio` -> 根级 `generate_audio`
- `auto_create_assets` -> 根级 `auto_create_assets`
- `reference_image_urls` / `reference_images` -> `content[]` 的 `image_url`
- `reference_video_urls` -> `content[]` 的 `video_url`
- `reference_audio_urls` -> `content[]` 的 `audio_url`

兼容行为：

- `seedance` 规格层本身没有正式常量化的 `resolution` 字段，所以 qiniu 实现按 provider 私有参数读取 `Params["resolution"]` 并原样透传到请求体。
- 参考媒体统一映射到 `content[]`，与 Qnagic 文档中的“多模态参考生视频”保持一致。

当前 Qiniu Seedance 后端会按 `content[]` 形式发送图片、视频、音频参考；`watermark` 仍未在本 provider 中透传。

## 素材审查

当请求包含 `reference_image_urls` / `reference_images` / `reference_video_urls` / `reference_audio_urls` 时，本 provider 默认会先调用 Qiniu 素材审查接口：

1. `POST https://openai.qiniu.com/v1/assets`
2. 轮询 `GET https://openai.qiniu.com/v1/assets/{qassetid}`
3. `approved` 后把原始 URL 替换为 `qasset://{qassetid}` 再提交视频任务

设置 `seedance.ParamAutoCreateAssets=true` 时，provider 不执行上述手工素材审查，而是保留原始 HTTP(S) URL，并在视频任务请求中发送 `auto_create_assets: true`，由平台在任务内部创建、审核和清理临时素材。

例外：`byteplus/dreamina-seedance-2-0-260128` 不支持素材审查，本 provider 会直接提交原始参考 URL。

可选配置：

- `QINIU_ASSETS_BASE_URL`：覆盖素材审查端点，例如 `https://openai.sufy.com`
- `QINIU_ASSET_GROUP_ID`：指定素材分组；不设置时由平台选择默认分组或自动创建
- `qiniu.ParamAssetGroupID`：通过 params 指定素材分组
- `qiniu.ParamAssetAutoReview=false`：关闭自动素材审查，直接提交原始 URL
- `qiniu.ParamAssetPollInterval`：素材轮询间隔，单位毫秒
- `qiniu.ParamAssetPollAttempts`：素材轮询次数
- `qiniu.ParamAssetReviewRetries`：素材审查可重试次数，默认 `2`。仅对基础设施、网络、超时、429/5xx、`[INFRA ASSET API ERROR]` / `Asset API` 等临时错误重试；内容审核拒绝不会重试

模型名约定：

- Qiniu 实际请求使用 `bytedance/doubao-seedance-2-0-260128`
- 海外 Dreamina 模型可直接使用 `byteplus/dreamina-seedance-2-0-260128`
- 为了不改通用 `spec/seedance`，provider 内部会把 `doubao-seedance-2-0-260128` 自动补成带 `bytedance/` 前缀的模型名

## 使用

```go
import (
    "context"
    "os"

    xai "github.com/goplus/xai/spec"
    "github.com/goplus/xai/spec/seedance"
    "github.com/goplus/xai/spec/seedance/provider/qiniu"
)

func main() {
    qiniu.Register(os.Getenv("QINIU_API_KEY"))

    ctx := context.Background()
    svc, _ := xai.New(ctx, "seedance://")
    op, _ := svc.Operation(xai.Model(seedance.ModelDoubaoSeedance20), xai.GenVideo)
    op.Params().(*seedance.Params).
        Set(seedance.ParamPrompt, "夕阳下的城市街道，电影感镜头缓慢推进").
        Set("resolution", "720p").
        Set(seedance.ParamRatio, "16:9").
        Set(seedance.ParamDuration, 5).
        Set(seedance.ParamGenerateAudio, true)
    _, _ = xai.CallSync(ctx, svc, op, svc.Options())
}
```
