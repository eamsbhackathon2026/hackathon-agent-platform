package domain

import "fmt"

// Generic wording kept for failures with no status: a transport error never reached
// the upstream service, so there is nothing to classify.
const toolErrorWithoutStatus = "Công cụ trả về trạng thái lỗi."

// MaxToolResultBytes bounds what one tool result may contribute to the prompt. An
// error body counts against the same budget as a successful one, so explaining the
// failure must not push the result past it.
const MaxToolResultBytes = 16 * 1024

// ToolErrorGuidance turns an unsuccessful HTTP response into an instruction the model
// can act on. Without it the model only sees the response body, which for an
// unhandled server error is the bare text "Internal Server Error" — indistinguishable
// from an outage. That ambiguity produced a real wrong answer: a request rejected for
// a bad customer id was reported to the customer as a temporary outage with advice to
// retry later, which could never succeed.
//
// The split that matters is who has to change something. A 4xx means the arguments
// were wrong and a corrected call can still work, so the model should fix them. A 5xx
// means the target system failed on input it accepted, so repeating the same call is
// pointless and the model must not invent the answer instead.
func ToolErrorGuidance(statusCode *int) string {
	if statusCode == nil {
		return toolErrorWithoutStatus
	}
	status := *statusCode
	switch {
	case status == 408 || status == 429:
		return fmt.Sprintf("Hệ thống đích tạm thời chưa phục vụ được yêu cầu này (HTTP %d). Chờ rồi thử lại, đừng gọi dồn.", status)
	case status >= 400 && status < 500:
		return fmt.Sprintf("Hệ thống đích từ chối yêu cầu (HTTP %d) vì dữ liệu gửi lên không hợp lệ hoặc không tồn tại. Kiểm tra lại tham số rồi gọi lại; gọi lại y nguyên sẽ vẫn lỗi.", status)
	case status >= 500:
		return fmt.Sprintf("Hệ thống đích gặp lỗi nội bộ (HTTP %d). Gọi lại y nguyên không khắc phục được. Nói rõ với người dùng là chưa tra cứu được, và tuyệt đối không tự suy đoán kết quả.", status)
	default:
		return fmt.Sprintf("Công cụ trả về trạng thái không xử lý được (HTTP %d).", status)
	}
}

// ToolErrorSummary states the same failure for a person reading the activity timeline,
// who needs to know where to look rather than what to do next. Keeping it separate from
// ToolErrorGuidance stops instructions addressed to the model from surfacing in the UI.
// With no status the wording stays as it was, because a transport failure carries its
// own reason in the result content.
func ToolErrorSummary(statusCode *int) string {
	if statusCode == nil {
		return "Công cụ trả về lỗi."
	}
	return fmt.Sprintf("Hệ thống đích trả về lỗi HTTP %d.", *statusCode)
}

// DescribeToolError prefixes the guidance to the response body so the model keeps the
// detail the target service provided — a 404 body naming the missing record is often
// exactly what the model needs to correct its arguments.
func DescribeToolError(statusCode *int, body string) string {
	guidance := ToolErrorGuidance(statusCode)
	if body == "" {
		return guidance
	}
	return guidance + "\nPhản hồi từ hệ thống đích: " + body
}
