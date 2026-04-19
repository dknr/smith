package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smith/llm"
	"smith/session"
	"smith/tools"
	"smith/types"

	"log/slog"
)

// fakeProvider satisfies llm.Provider by feeding predetermined tokens or calls.
type fakeProvider struct {
	tokens    []string
	callText  string
	callTools []types.ToolCall
	callErr   error
}

func (f *fakeProvider) Complete(ctx context.Context, messages []types.Message) (<-chan string, error) {
	if f.callErr != nil {
		return nil, f.callErr
	}
	ch := make(chan string, len(f.tokens))
	for _, t := range f.tokens {
		ch <- t
	}
	close(ch)
	return ch, nil
}

func (f *fakeProvider) Call(ctx context.Context, messages []types.Message, toolDefs []types.ToolDef) (llm.CallResult, error) {
	if f.callErr != nil {
		return llm.CallResult{}, f.callErr
	}
	if len(f.callTools) > 0 {
		tools := f.callTools
		f.callTools = nil
		return llm.CallResult{ToolCalls: tools}, nil
	}
	return llm.CallResult{Text: f.callText}, nil
}

func newFakeAgent(callText string, callTools []types.ToolCall) *Agent {
	fp := &fakeProvider{callText: callText, callTools: callTools}
	reg := tools.NewRegistry()
	sess, _ := session.New()
	logger := slog.Default()
	return New(fp, reg, sess, logger)
}

func newFakeAgentWithErr(callErr error) *Agent {
	fp := &fakeProvider{callErr: callErr}
	reg := tools.NewRegistry()
	sess, _ := session.New()
	logger := slog.Default()
	return New(fp, reg, sess, logger)
}

func TestHistory_empty(t *testing.T) {
	a := newFakeAgent("", nil)
	h := a.History()
	if len(h) != 0 {
		t.Errorf("expected empty history, got %d messages", len(h))
	}
}

func TestProcessMessage_singleTurn(t *testing.T) {
	a := newFakeAgent("Hello world", nil)

	respCh, err := a.ProcessMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != len("Hello world")+1 {
		t.Fatalf("expected %d responses (one per char + done), got %d", len("Hello world")+1, len(responses))
	}
}

func TestProcessMessage_toolCall(t *testing.T) {
	a := newFakeAgent("", []types.ToolCall{
		{ID: "call_1", Name: "time", Arguments: "{}"},
	})

	respCh, err := a.ProcessMessage(context.Background(), "what time is it")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses (tool_call + text), got %d", len(responses))
	}
	if responses[0].Role != "tool_call" {
		t.Errorf("first response role = %q, want %q", responses[0].Role, "tool_call")
	}
	if !strings.Contains(responses[0].Content, "time") {
		t.Errorf("tool_call content should mention tool name, got %q", responses[0].Content)
	}
	if !responses[len(responses)-1].Done {
		t.Error("expected final response to have done=true")
	}
}

func TestProcessMessage_toolCallWithError(t *testing.T) {
	a := newFakeAgent("", []types.ToolCall{
		{ID: "call_1", Name: "view", Arguments: `{"path":"/no/such/file"}`},
	})

	respCh, err := a.ProcessMessage(context.Background(), "read the file")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) < 2 {
		t.Fatalf("expected at least 2 responses (tool_call + text), got %d", len(responses))
	}
	if responses[0].Role != "tool_call" {
		t.Errorf("first response role = %q, want %q", responses[0].Role, "tool_call")
	}
	if !responses[len(responses)-1].Done {
		t.Error("expected final response to have done=true")
	}
}

func TestProcessMessage_providerError(t *testing.T) {
	a := newFakeAgentWithErr(context.Canceled)

	respCh, err := a.ProcessMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if !responses[0].Done {
		t.Error("expected done=true for error response")
	}
	if !strings.Contains(responses[0].Content, "context canceled") {
		t.Errorf("expected error content, got %q", responses[0].Content)
	}
}

func TestProcessMessage_contextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	blocking := &fakeProvider{}
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	a := New(blocking, tools.NewRegistry(), nil, slog.Default())
	respCh, err := a.ProcessMessage(ctx, "hi")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	select {
	case <-respCh:
	case <-time.After(500 * time.Millisecond):
	}
}

func TestHistory_afterProcessMessage(t *testing.T) {
	a := newFakeAgent("answer", nil)

	respCh, err := a.ProcessMessage(context.Background(), "question")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}
	for range respCh {
	}

	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages in history, got %d", len(h))
	}
	if h[0].Role != "user" || h[0].Content != "question" {
		t.Errorf("history[0] = %+v, want {user, question}", h[0])
	}
	if h[1].Role != "assistant" || h[1].Content != "answer" {
		t.Errorf("history[1] = %+v, want {assistant, answer}", h[1])
	}
}

func TestHistory_defensiveCopy(t *testing.T) {
	a := newFakeAgent("hi", nil)

	respCh, _ := a.ProcessMessage(context.Background(), "hello")
	for range respCh {
	}

	h1 := a.History()
	h1[0].Content = "mutated"

	h2 := a.History()
	if h2[0].Content == "mutated" {
		t.Error("History should return a defensive copy")
	}
}

func TestHistory_toolCallLoop(t *testing.T) {
	a := newFakeAgent("", []types.ToolCall{{ID: "call_1", Name: "time", Arguments: "{}"}})

	respCh, _ := a.ProcessMessage(context.Background(), "what time is it")
	for range respCh {
	}

	h := a.History()
	// user message, assistant tool call, tool result, assistant text
	if len(h) != 4 {
		t.Fatalf("expected 4 messages after tool loop, got %d", len(h))
	}
	if h[0].Role != "user" {
		t.Errorf("history[0] role = %q, want %q", h[0].Role, "user")
	}
	if h[1].Role != "assistant" || len(h[1].ToolCalls) != 1 {
		t.Errorf("history[1] should have tool call, got %+v", h[1])
	}
	if h[2].Role != "tool" {
		t.Errorf("history[2] role = %q, want %q", h[2].Role, "tool")
	}
	if h[3].Role != "assistant" {
		t.Errorf("history[3] role = %q, want %q", h[3].Role, "assistant")
	}
}

func TestHistory_toolCallWithFileView(t *testing.T) {
	tmp := t.TempDir()
	testFile := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	a := newFakeAgent("", []types.ToolCall{{ID: "call_1", Name: "view", Arguments: `{"path":"` + testFile + `"}`}})

	respCh, _ := a.ProcessMessage(context.Background(), "read the file")
	for range respCh {
	}

	h := a.History()
	if len(h) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(h))
	}
	if h[2].Role != "tool" || h[2].Content != "hello world" {
		t.Errorf("tool result = %q, want %q", h[2].Content, "hello world")
	}
}

func TestNew_providerStored(t *testing.T) {
	fp := &fakeProvider{callText: "test"}
	reg := tools.NewRegistry()
	sess, _ := session.New()
	a := New(fp, reg, sess, slog.Default())
	if a.provider != fp {
		t.Error("provider not stored correctly")
	}
	if a.executor == nil {
		t.Error("executor not stored correctly")
	}
	if a.session == nil {
		t.Error("session not stored correctly")
	}
}

func TestProcessMessage_toolCallResponse(t *testing.T) {
	a := newFakeAgent("", []types.ToolCall{{ID: "call_1", Name: "time", Arguments: "{}"}})

	respCh, _ := a.ProcessMessage(context.Background(), "what time is it")

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) < 2 {
		t.Fatalf("expected tool_call + text responses, got %d", len(responses))
	}
	if responses[0].Role != "tool_call" {
		t.Errorf("first response role = %q, want %q", responses[0].Role, "tool_call")
	}
	if responses[0].Content != "time()" {
		t.Errorf("tool_call content = %q, want %q", responses[0].Content, "time()")
	}
	if !responses[len(responses)-1].Done {
		t.Error("expected final response to have done=true")
	}
}

func TestFormatToolCall_noArgs(t *testing.T) {
	got := formatToolCall("time", "{}")
	if got != "time()" {
		t.Errorf("formatToolCall(time, '{}') = %q, want %q", got, "time()")
	}
}

func TestFormatToolCall_noArgsEmpty(t *testing.T) {
	got := formatToolCall("time", "")
	if got != "time()" {
		t.Errorf("formatToolCall(time, '') = %q, want %q", got, "time()")
	}
}

func TestFormatToolCall_withArgs(t *testing.T) {
	got := formatToolCall("view", `{"path":"foo/output.txt"}`)
	want := `view(path="foo/output.txt")`
	if got != want {
		t.Errorf("formatToolCall(view, ...) = %q, want %q", got, want)
	}
}

func TestFormatToolCall_multipleArgs(t *testing.T) {
	got := formatToolCall("list", `{"path":"foo/bar","all":false}`)
	want := `list(path="foo/bar", all=false)`
	if got != want && got != `list(all=false, path="foo/bar")` {
		t.Errorf("formatToolCall(list, ...) = %q, want %q", got, want)
	}
}

func TestFormatToolCall_invalidJSON(t *testing.T) {
	got := formatToolCall("view", "not json")
	if got != "view(not json)" {
		t.Errorf("formatToolCall with invalid JSON should fall back to raw string, got %q", got)
	}
}

func TestProcessMessage_errorResponse(t *testing.T) {
	a := newFakeAgentWithErr(fmt.Errorf("api error"))

	respCh, _ := a.ProcessMessage(context.Background(), "hi")

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if !responses[0].Done {
		t.Error("expected done=true for error response")
	}
	if responses[0].Role != "error" {
		t.Errorf("expected role=error, got %q", responses[0].Role)
	}
}
