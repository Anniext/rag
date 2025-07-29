package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"pumppill/rag/core"
)

func TestProtocolVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  ProtocolVersion
		expected string
	}{
		{
			name:     "basic version",
			version:  ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			expected: "1.0.0",
		},
		{
			name:     "complex version",
			version:  ProtocolVersion{Major: 2, Minor: 3, Patch: 4},
			expected: "2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.String()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestProtocolVersionCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		version1   ProtocolVersion
		version2   ProtocolVersion
		compatible bool
	}{
		{
			name:       "same version",
			version1:   ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			version2:   ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			compatible: true,
		},
		{
			name:       "backward compatible",
			version1:   ProtocolVersion{Major: 1, Minor: 1, Patch: 0},
			version2:   ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			compatible: true,
		},
		{
			name:       "not backward compatible",
			version1:   ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			version2:   ProtocolVersion{Major: 1, Minor: 1, Patch: 0},
			compatible: false,
		},
		{
			name:       "different major version",
			version1:   ProtocolVersion{Major: 2, Minor: 0, Patch: 0},
			version2:   ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
			compatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version1.IsCompatible(tt.version2)
			if result != tt.compatible {
				t.Errorf("expected %v, got %v", tt.compatible, result)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name        string
		versionStr  string
		expected    ProtocolVersion
		expectError bool
	}{
		{
			name:       "valid version",
			versionStr: "1.0.0",
			expected:   ProtocolVersion{Major: 1, Minor: 0, Patch: 0},
		},
		{
			name:       "complex version",
			versionStr: "2.3.4",
			expected:   ProtocolVersion{Major: 2, Minor: 3, Patch: 4},
		},
		{
			name:        "invalid format",
			versionStr:  "1.0",
			expectError: true,
		},
		{
			name:        "non-numeric",
			versionStr:  "a.b.c",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseVersion(tt.versionStr)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMessageTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		messageType MessageType
		valid       bool
	}{
		{"request", MessageTypeRequest, true},
		{"response", MessageTypeResponse, true},
		{"notification", MessageTypeNotification, true},
		{"error", MessageTypeError, true},
		{"invalid", MessageType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.messageType.IsValid()
			if result != tt.valid {
				t.Errorf("expected %v, got %v", tt.valid, result)
			}
		})
	}
}

func TestJSONMessageParser(t *testing.T) {
	logger := &SimpleMockLogger{}
	parser := NewJSONMessageParser(logger)

	t.Run("parse valid request", func(t *testing.T) {
		data := []byte(`{"type":"request","id":"123","method":"test","params":{"key":"value"}}`)
		message, err := parser.ParseMessage(data)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if message.Type != "request" {
			t.Errorf("expected type 'request', got %s", message.Type)
		}
		if message.ID != "123" {
			t.Errorf("expected ID '123', got %s", message.ID)
		}
		if message.Method != "test" {
			t.Errorf("expected method 'test', got %s", message.Method)
		}
	})

	t.Run("parse invalid JSON", func(t *testing.T) {
		data := []byte(`{"type":"request","id":}`)
		_, err := parser.ParseMessage(data)
		if err == nil {
			t.Error("expected error but got none")
		}

		protocolErr, ok := err.(*ProtocolError)
		if !ok {
			t.Errorf("expected ProtocolError, got %T", err)
		}
		if protocolErr.Code != ErrorCodeParseError {
			t.Errorf("expected error code %d, got %d", ErrorCodeParseError, protocolErr.Code)
		}
	})

	t.Run("serialize valid message", func(t *testing.T) {
		message := &core.MCPMessage{
			Type:   "request",
			ID:     "123",
			Method: "test",
			Params: map[string]any{"key": "value"},
		}

		data, err := parser.SerializeMessage(message)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// 验证可以重新解析
		parsed, err := parser.ParseMessage(data)
		if err != nil {
			t.Errorf("failed to parse serialized message: %v", err)
			return
		}

		if parsed.Type != message.Type {
			t.Errorf("expected type %s, got %s", message.Type, parsed.Type)
		}
	})
}

func TestMessageValidation(t *testing.T) {
	logger := &SimpleMockLogger{}
	parser := NewJSONMessageParser(logger)

	tests := []struct {
		name        string
		message     *core.MCPMessage
		expectError bool
		errorCode   int
	}{
		{
			name: "valid request",
			message: &core.MCPMessage{
				Type:   "request",
				ID:     "123",
				Method: "test",
			},
			expectError: false,
		},
		{
			name: "request without ID",
			message: &core.MCPMessage{
				Type:   "request",
				Method: "test",
			},
			expectError: true,
			errorCode:   ErrorCodeInvalidRequest,
		},
		{
			name: "request without method",
			message: &core.MCPMessage{
				Type: "request",
				ID:   "123",
			},
			expectError: true,
			errorCode:   ErrorCodeInvalidRequest,
		},
		{
			name: "valid response with result",
			message: &core.MCPMessage{
				Type:   "response",
				ID:     "123",
				Result: "success",
			},
			expectError: false,
		},
		{
			name: "valid response with error",
			message: &core.MCPMessage{
				Type: "response",
				ID:   "123",
				Error: &core.MCPError{
					Code:    500,
					Message: "error",
				},
			},
			expectError: false,
		},
		{
			name: "response without result or error",
			message: &core.MCPMessage{
				Type: "response",
				ID:   "123",
			},
			expectError: true,
			errorCode:   ErrorCodeInvalidRequest,
		},
		{
			name: "response with both result and error",
			message: &core.MCPMessage{
				Type:   "response",
				ID:     "123",
				Result: "success",
				Error: &core.MCPError{
					Code:    500,
					Message: "error",
				},
			},
			expectError: true,
			errorCode:   ErrorCodeInvalidRequest,
		},
		{
			name: "valid notification",
			message: &core.MCPMessage{
				Type:   "notification",
				Method: "test",
			},
			expectError: false,
		},
		{
			name: "notification with ID",
			message: &core.MCPMessage{
				Type:   "notification",
				ID:     "123",
				Method: "test",
			},
			expectError: true,
			errorCode:   ErrorCodeInvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateMessage(tt.message)
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
					return
				}

				protocolErr, ok := err.(*ProtocolError)
				if !ok {
					t.Errorf("expected ProtocolError, got %T", err)
					return
				}

				if protocolErr.Code != tt.errorCode {
					t.Errorf("expected error code %d, got %d", tt.errorCode, protocolErr.Code)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestVersionNegotiator(t *testing.T) {
	logger := &SimpleMockLogger{}
	supportedVersions := []ProtocolVersion{
		{Major: 1, Minor: 0, Patch: 0},
		{Major: 1, Minor: 1, Patch: 0},
	}
	negotiator := NewVersionNegotiator(supportedVersions, logger)

	t.Run("successful negotiation", func(t *testing.T) {
		clientVersions := []ProtocolVersion{
			{Major: 1, Minor: 0, Patch: 0},
			{Major: 1, Minor: 1, Patch: 0},
		}

		version, err := negotiator.NegotiateVersion(clientVersions)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		expected := ProtocolVersion{Major: 1, Minor: 1, Patch: 0}
		if version != expected {
			t.Errorf("expected version %v, got %v", expected, version)
		}
	})

	t.Run("no compatible version", func(t *testing.T) {
		clientVersions := []ProtocolVersion{
			{Major: 2, Minor: 0, Patch: 0},
		}

		_, err := negotiator.NegotiateVersion(clientVersions)
		if err == nil {
			t.Error("expected error but got none")
			return
		}

		protocolErr, ok := err.(*ProtocolError)
		if !ok {
			t.Errorf("expected ProtocolError, got %T", err)
			return
		}

		if protocolErr.Code != ErrorCodeVersionError {
			t.Errorf("expected error code %d, got %d", ErrorCodeVersionError, protocolErr.Code)
		}
	})

	t.Run("empty client versions", func(t *testing.T) {
		clientVersions := []ProtocolVersion{}

		_, err := negotiator.NegotiateVersion(clientVersions)
		if err == nil {
			t.Error("expected error but got none")
			return
		}
	})
}

func TestMessageRouter(t *testing.T) {
	logger := &SimpleMockLogger{}
	metrics := &SimpleMockMetricsCollector{}
	router := NewMessageRouter(logger, metrics)

	handler := &SimpleMockHandler{result: "test result"}
	version := ProtocolVersion{Major: 1, Minor: 0, Patch: 0}

	t.Run("register and route message", func(t *testing.T) {
		err := router.RegisterRoute("test.method", handler, nil, "Test method", version)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		message := &core.MCPMessage{
			Type:   "request",
			ID:     "123",
			Method: "test.method",
			Params: map[string]any{"key": "value"},
		}

		ctx := context.Background()
		response, err := router.RouteMessage(ctx, message, "conn1", version)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if response.ID != "123" {
			t.Errorf("expected ID '123', got %s", response.ID)
		}

		if response.Result != "test result" {
			t.Errorf("expected result 'test result', got %v", response.Result)
		}
	})

	t.Run("route non-existent method", func(t *testing.T) {
		message := &core.MCPMessage{
			Type:   "request",
			ID:     "123",
			Method: "non.existent",
		}

		ctx := context.Background()
		response, err := router.RouteMessage(ctx, message, "conn1", version)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if response.Error == nil {
			t.Error("expected error but got none")
			return
		}

		if response.Error.Code != ErrorCodeMethodNotFound {
			t.Errorf("expected error code %d, got %d", ErrorCodeMethodNotFound, response.Error.Code)
		}
	})

	t.Run("version incompatibility", func(t *testing.T) {
		message := &core.MCPMessage{
			Type:   "request",
			ID:     "123",
			Method: "test.method",
		}

		incompatibleVersion := ProtocolVersion{Major: 2, Minor: 0, Patch: 0}
		ctx := context.Background()
		response, err := router.RouteMessage(ctx, message, "conn1", incompatibleVersion)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if response.Error == nil {
			t.Error("expected error but got none")
			return
		}

		if response.Error.Code != ErrorCodeVersionError {
			t.Errorf("expected error code %d, got %d", ErrorCodeVersionError, response.Error.Code)
		}
	})
}

func TestMessageDispatcher(t *testing.T) {
	logger := &SimpleMockLogger{}
	metrics := &SimpleMockMetricsCollector{}
	parser := NewJSONMessageParser(logger)
	router := NewMessageRouter(logger, metrics)
	negotiator := NewVersionNegotiator(SupportedVersions, logger)
	dispatcher := NewMessageDispatcher(parser, router, negotiator, logger, metrics)

	// 注册测试处理器
	handler := &SimpleMockHandler{result: "test result"}
	version := ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
	router.RegisterRoute("test.method", handler, nil, "Test method", version)

	t.Run("dispatch request message", func(t *testing.T) {
		requestData := []byte(`{"type":"request","id":"123","method":"test.method","params":{"key":"value"}}`)

		ctx := context.Background()
		responseData, err := dispatcher.DispatchMessage(ctx, requestData, "conn1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// 解析响应
		var responseMessage core.MCPMessage
		err = json.Unmarshal(responseData, &responseMessage)
		if err != nil {
			t.Errorf("failed to parse response: %v", err)
			return
		}

		if responseMessage.Type != "response" {
			t.Errorf("expected type 'response', got %s", responseMessage.Type)
		}

		if responseMessage.ID != "123" {
			t.Errorf("expected ID '123', got %s", responseMessage.ID)
		}

		if responseMessage.Result != "test result" {
			t.Errorf("expected result 'test result', got %v", responseMessage.Result)
		}
	})

	t.Run("dispatch notification message", func(t *testing.T) {
		notificationData := []byte(`{"type":"notification","method":"test.notification","params":{"key":"value"}}`)

		ctx := context.Background()
		responseData, err := dispatcher.DispatchMessage(ctx, notificationData, "conn1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// 通知不应该有响应
		if responseData != nil {
			t.Errorf("expected no response for notification, got %s", string(responseData))
		}
	})

	t.Run("dispatch invalid JSON", func(t *testing.T) {
		invalidData := []byte(`{"type":"request","id":}`)

		ctx := context.Background()
		responseData, err := dispatcher.DispatchMessage(ctx, invalidData, "conn1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// 解析错误响应
		var responseMessage core.MCPMessage
		err = json.Unmarshal(responseData, &responseMessage)
		if err != nil {
			t.Errorf("failed to parse error response: %v", err)
			return
		}

		if responseMessage.Error == nil {
			t.Error("expected error but got none")
			return
		}

		if responseMessage.Error.Code != ErrorCodeParseError {
			t.Errorf("expected error code %d, got %d", ErrorCodeParseError, responseMessage.Error.Code)
		}
	})
}

func TestProtocolHandler(t *testing.T) {
	logger := &SimpleMockLogger{}
	metrics := &SimpleMockMetricsCollector{}
	handler := NewProtocolHandler(logger, metrics)

	// 注册测试处理器
	mockHandler := &SimpleMockHandler{result: "test result"}
	err := handler.RegisterHandler("test.method", mockHandler, nil, "Test method")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	t.Run("process request", func(t *testing.T) {
		requestData := []byte(`{"type":"request","id":"123","method":"test.method","params":{"key":"value"}}`)

		ctx := context.Background()
		responseData, err := handler.ProcessMessage(ctx, requestData, "conn1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		// 解析响应
		var responseMessage core.MCPMessage
		err = json.Unmarshal(responseData, &responseMessage)
		if err != nil {
			t.Errorf("failed to parse response: %v", err)
			return
		}

		if responseMessage.Result != "test result" {
			t.Errorf("expected result 'test result', got %v", responseMessage.Result)
		}
	})

	t.Run("get supported methods", func(t *testing.T) {
		methods := handler.GetSupportedMethods()

		// 应该包含注册的方法和内置方法
		expectedMethods := []string{"test.method", "protocol.negotiate"}
		for _, expected := range expectedMethods {
			found := false
			for _, method := range methods {
				if method == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected method %s not found in %v", expected, methods)
			}
		}
	})

	t.Run("get supported versions", func(t *testing.T) {
		versions := handler.GetSupportedVersions()
		if len(versions) == 0 {
			t.Error("expected at least one supported version")
		}

		// 应该包含当前版本
		found := false
		for _, version := range versions {
			if version == CurrentProtocolVersion {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("current version %v not found in supported versions %v", CurrentProtocolVersion, versions)
		}
	})
}

func TestProtocolError(t *testing.T) {
	err := NewProtocolError(ErrorCodeInvalidRequest, "Invalid request", "test data")

	if err.Code != ErrorCodeInvalidRequest {
		t.Errorf("expected code %d, got %d", ErrorCodeInvalidRequest, err.Code)
	}

	if err.Message != "Invalid request" {
		t.Errorf("expected message 'Invalid request', got %s", err.Message)
	}

	if err.Data != "test data" {
		t.Errorf("expected data 'test data', got %v", err.Data)
	}

	expectedErrorString := "protocol error -32600: Invalid request"
	if err.Error() != expectedErrorString {
		t.Errorf("expected error string '%s', got '%s'", expectedErrorString, err.Error())
	}
}

// 基准测试
func BenchmarkJSONMessageParser_ParseMessage(b *testing.B) {
	logger := &SimpleMockLogger{}
	parser := NewJSONMessageParser(logger)
	data := []byte(`{"type":"request","id":"123","method":"test","params":{"key":"value"}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.ParseMessage(data)
		if err != nil {
			b.Errorf("unexpected error: %v", err)
		}
	}
}

func BenchmarkJSONMessageParser_SerializeMessage(b *testing.B) {
	logger := &SimpleMockLogger{}
	parser := NewJSONMessageParser(logger)
	message := &core.MCPMessage{
		Type:   "request",
		ID:     "123",
		Method: "test",
		Params: map[string]any{"key": "value"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := parser.SerializeMessage(message)
		if err != nil {
			b.Errorf("unexpected error: %v", err)
		}
	}
}

func BenchmarkMessageRouter_RouteMessage(b *testing.B) {
	logger := &SimpleMockLogger{}
	metrics := &SimpleMockMetricsCollector{}
	router := NewMessageRouter(logger, metrics)

	handler := &SimpleMockHandler{result: "test result"}
	version := ProtocolVersion{Major: 1, Minor: 0, Patch: 0}
	router.RegisterRoute("test.method", handler, nil, "Test method", version)

	message := &core.MCPMessage{
		Type:   "request",
		ID:     "123",
		Method: "test.method",
		Params: map[string]any{"key": "value"},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := router.RouteMessage(ctx, message, "conn1", version)
		if err != nil {
			b.Errorf("unexpected error: %v", err)
		}
	}
}
