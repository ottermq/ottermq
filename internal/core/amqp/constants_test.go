package amqp

import "testing"

// TestReplyCodeConstants tests that reply code constants match AMQP 0-9-1 specification
func TestReplyCodeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant ReplyCode
		expected uint16
	}{
		{"REPLY_SUCCESS", REPLY_SUCCESS, 200},
		{"CONTENT_TOO_LARGE", CONTENT_TOO_LARGE, 311},
		{"NO_ROUTE", NO_ROUTE, 312},
		{"NO_CONSUMERS", NO_CONSUMERS, 313},
		{"CONNECTION_FORCED", CONNECTION_FORCED, 320},
		{"INVALID_PATH", INVALID_PATH, 402},
		{"ACCESS_REFUSED", ACCESS_REFUSED, 403},
		{"NOT_FOUND", NOT_FOUND, 404},
		{"RESOURCE_LOCKED", RESOURCE_LOCKED, 405},
		{"PRECONDITION_FAILED", PRECONDITION_FAILED, 406},
		{"FRAME_ERROR", FRAME_ERROR, 501},
		{"SYNTAX_ERROR", SYNTAX_ERROR, 502},
		{"COMMAND_INVALID", COMMAND_INVALID, 503},
		{"CHANNEL_ERROR", CHANNEL_ERROR, 504},
		{"UNEXPECTED_FRAME", UNEXPECTED_FRAME, 505},
		{"RESOURCE_ERROR", RESOURCE_ERROR, 506},
		{"NOT_ALLOWED", NOT_ALLOWED, 530},
		{"NOT_IMPLEMENTED", NOT_IMPLEMENTED, 540},
		{"INTERNAL_ERROR", INTERNAL_ERROR, 541},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if uint16(tt.constant) != tt.expected {
				t.Errorf("%s = %d, expected %d", tt.name, uint16(tt.constant), tt.expected)
			}
		})
	}
}

// TestReplyTextMap tests that all reply codes have corresponding text messages
func TestReplyTextMap(t *testing.T) {
	tests := []struct {
		code         ReplyCode
		expectedText string
	}{
		{REPLY_SUCCESS, "REPLY_SUCCESS"},
		{CONTENT_TOO_LARGE, "CONTENT_TOO_LARGE"},
		{NO_ROUTE, "NO_ROUTE"},
		{NO_CONSUMERS, "NO_CONSUMERS"},
		{CONNECTION_FORCED, "CONNECTION_FORCED"},
		{INVALID_PATH, "INVALID_PATH"},
		{ACCESS_REFUSED, "ACCESS_REFUSED"},
		{NOT_FOUND, "NOT_FOUND"},
		{RESOURCE_LOCKED, "RESOURCE_LOCKED"},
		{PRECONDITION_FAILED, "PRECONDITION_FAILED"},
		{FRAME_ERROR, "FRAME_ERROR"},
		{SYNTAX_ERROR, "SYNTAX_ERROR"},
		{COMMAND_INVALID, "COMMAND_INVALID"},
		{CHANNEL_ERROR, "CHANNEL_ERROR"},
		{UNEXPECTED_FRAME, "UNEXPECTED_FRAME"},
		{RESOURCE_ERROR, "RESOURCE_ERROR"},
		{NOT_ALLOWED, "NOT_ALLOWED"},
		{NOT_IMPLEMENTED, "NOT_IMPLEMENTED"},
		{INTERNAL_ERROR, "INTERNAL_ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedText, func(t *testing.T) {
			if text, exists := ReplyText[tt.code]; !exists {
				t.Errorf("ReplyText missing for code %d", tt.code)
			} else if text != tt.expectedText {
				t.Errorf("ReplyText[%d] = %q, expected %q", tt.code, text, tt.expectedText)
			}
		})
	}
}

// TestReplyTextMapCompleteness verifies that all defined reply codes have text messages
func TestReplyTextMapCompleteness(t *testing.T) {
	expectedCodes := []ReplyCode{
		REPLY_SUCCESS, CONTENT_TOO_LARGE, NO_ROUTE, NO_CONSUMERS,
		CONNECTION_FORCED, INVALID_PATH, ACCESS_REFUSED, NOT_FOUND,
		RESOURCE_LOCKED, PRECONDITION_FAILED, FRAME_ERROR, SYNTAX_ERROR,
		COMMAND_INVALID, CHANNEL_ERROR, UNEXPECTED_FRAME, RESOURCE_ERROR,
		NOT_ALLOWED, NOT_IMPLEMENTED, INTERNAL_ERROR,
	}

	if len(ReplyText) != len(expectedCodes) {
		t.Errorf("ReplyText map has %d entries, expected %d", len(ReplyText), len(expectedCodes))
	}

	for _, code := range expectedCodes {
		if _, exists := ReplyText[code]; !exists {
			t.Errorf("ReplyText missing entry for code %d", code)
		}
	}
}
