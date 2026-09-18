-- +goose Up
-- Nhãn hiện cho người dùng cuối trong lúc trợ lý chạy công cụ. Tách khỏi
-- display_name vì hai chỗ đọc khác nhau: display_name là tên trong danh mục
-- công cụ của người vận hành, còn nhãn này chạy qua mắt khách hàng giữa một
-- câu trả lời đang hình thành, nên câu chữ phải ngắn và ở thì đang diễn ra.
-- Công cụ có sẵn giữ chuỗi rỗng và rơi về display_name, nên không cần backfill.
ALTER TABLE tools ADD COLUMN step_label text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE tools DROP COLUMN step_label;
