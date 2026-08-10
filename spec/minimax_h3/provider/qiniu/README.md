# Qiniu MiniMax-H3 provider

The provider uses the FAL Queue-compatible API at `https://api.qnaigc.com`.
It submits to one of the following paths and polls the corresponding request
status/result paths:

- `/queue/minimax/h3/text-to-video`
- `/queue/minimax/h3/image-to-video`
- `/queue/minimax/h3/reference-to-video`

Authentication is `Authorization: Key <api-key>`. Set `QINIU_API_KEY` or pass
the key to `NewService`.
