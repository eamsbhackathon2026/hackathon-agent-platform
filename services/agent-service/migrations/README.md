# Migration cơ sở dữ liệu

Các migration hiện tại xây schema một workspace theo thứ tự:

- `00001_identity.sql`: `users`, `refresh_tokens`, `api_keys`; email duy nhất
  không phân biệt hoa/thường, constraints role/status/scopes và refresh rotation.
- `00002_catalog.sql`: kết nối AI và trợ lý; archive trợ lý, revision CAS và khóa
  ngoại bảo toàn lịch sử cấu hình.
- `00003_runs.sql`: `sessions`, `runs`, `messages`, `spans`; message sequence tăng
  nguyên tử, lifecycle/owner constraints, cursor indexes và soft-delete session.
  Unique index có điều kiện giữ `session_key` duy nhất theo API key trong các
  session còn hoạt động, đồng thời cho phép tái dùng key sau khi session bị xóa.
- `00004_tools.sql`: HTTP/MCP tools, secret headers và binding với trợ lý.
- `00005_async.sql`: hàng đợi run, idempotency và webhook delivery bền vững.
- `00006_greennode_provider.sql`: mở rộng provider kind với GreenNode trong khi
  giữ nguyên quy tắc base URL bắt buộc của OpenAI-compatible.
- `00007_skills.sql`: thư viện skill dùng chung và binding nhiều-nhiều với trợ lý;
  nội dung Markdown chuẩn hóa được lưu trong Postgres và binding tự xóa theo FK.

Tạo migration mới bằng `make migrate-new name=...`. Migration `Down` xóa dữ liệu
của đúng phase theo thứ tự ngược; chỉ dùng khi chủ động rollback môi trường phù hợp.

File SQL được nhúng vào binary qua package `migrations`; không đổi thứ tự hoặc
nội dung migration đã áp dụng vào môi trường có dữ liệu.

Integration tests dùng `pgtest.NewPool(t)` với schema UUID riêng cho từng test,
áp dụng migration và chỉ dọn schema đó. Khi có `TEST_DATABASE_URL`, helper tái
sử dụng server được chỉ định; nếu không, khởi động container `postgres:18`.
Helper không tự dùng `DATABASE_URL`, không truncate hoặc reset schema ứng dụng.
