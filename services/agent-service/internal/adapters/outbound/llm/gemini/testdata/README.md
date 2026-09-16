# Fixture Gemini

`tool-turn.jsonl` là dữ liệu **tổng hợp**, không phải response được ghi lại từ tài khoản thật.
Mỗi dòng là JSON của một SSE `data` event theo wire format chính thức:

- https://ai.google.dev/api/generate-content#method:-models.streamgeneratecontent
- https://ai.google.dev/gemini-api/docs/function-calling
- https://ai.google.dev/gemini-api/docs/thinking (ghi chú về chữ ký của `generateContent`)

Các chữ ký Base64 chỉ là giá trị giả để kiểm tra round-trip byte; không dùng gọi API thật.
Fixture cố ý có nhiều lời gọi cùng tên không có ID, ID do provider cấp, candidate khác,
thought riêng tư, chữ ký trên text và part chỉ có chữ ký. Kiểm thử gửi fixture qua
`httptest` và SDK `google.golang.org/genai v1.71.0` thật.

Metadata lưu toàn bộ part mà SDK nhận diện và giữ thứ tự. Field wire chưa được SDK
ghim hỗ trợ có thể đã bị SDK loại bỏ trước khi adapter nhận dữ liệu.

SDK bỏ trường text có giá trị rỗng khi serialize. Hook trước JSON encode khôi phục
`text: ""` cho assistant part thiếu data, giữ chữ ký tại đúng vị trí. Kiểm thử
`gemini_replay_test.go` kiểm tra JSON request thực qua SDK cho cả signed/unsigned
empty text. Không ghép chữ ký vào part khác; xem
[FAQ về chữ ký trong stream](https://ai.google.dev/gemini-api/docs/generate-content/thought-signatures).

Kiểm thử `-tags live` dùng `GEMINI_API_KEY`, tùy chọn `GEMINI_MODEL`; mặc định
`gemini-3.8-flash` theo ví dụ Go chính thức đã đối chiếu ngày 2026-09-14.
Live ListModels và tool calling hai lượt đã đạt với model này ngày 2026-09-14.
Fixture trên vẫn là dữ liệu tổng hợp, không lưu nội dung hay chữ ký từ lần gọi live.
