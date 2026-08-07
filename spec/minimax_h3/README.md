# MiniMax-H3

`spec/minimax_h3` defines the xai asynchronous `GenVideo` contract for
Qiniu MaaS MiniMax-H3 (`minimax/minimax-h3`). The request mode maps to the
three upstream Queue endpoints:

- `text_to_video` -> text-to-video
- `image_to_video` and `start_end_to_video` -> image-to-video
- `multi_ref_to_video` -> reference-to-video

The provider backend owns HTTP transport, status polling, result retrieval,
and the Qiniu `Authorization: Key <api-key>` header.
