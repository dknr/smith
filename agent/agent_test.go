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
	tokens       []string
	callText     string
	callTexts    []string
	callTools    []types.ToolCall
	callErr      error
	callCount    int
	callErrAfter int // number of successful calls before returning error
}

func (f *fakeProvider) Complete(ctx context.Context, messages []types.Message) (<-chan string, error) {
	if f.callErr != nil && f.callCount >= f.callErrAfter {
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
	if f.callErr != nil && f.callCount >= f.callErrAfter {
		return llm.CallResult{}, f.callErr
	}
	if len(f.callTools) > 0 {
		tools := f.callTools
		f.callTools = nil
		return llm.CallResult{ToolCalls: tools}, nil
	}
	f.callCount++
	if f.callCount <= len(f.callTexts) {
		return llm.CallResult{Text: f.callTexts[f.callCount-1]}, nil
	}
	return llm.CallResult{Text: f.callText}, nil
}

func newFakeAgent(callText string, callTools []types.ToolCall) *Agent {
	fp := &fakeProvider{callText: callText, callTools: callTools}
	reg := tools.NewRegistry()
	sess, _ := session.New()
	logger := slog.Default()
	return New(fp, reg, sess, logger, nil)
}

func newFakeAgentWithErr(callErr error) *Agent {
	fp := &fakeProvider{callErr: callErr}
	reg := tools.NewRegistry()
	sess, _ := session.New()
	logger := slog.Default()
	return New(fp, reg, sess, logger, nil)
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

	if len(responses) != 1 {
		t.Fatalf("expected 1 response (done), got %d", len(responses))
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

	a := New(blocking, tools.NewRegistry(), nil, slog.Default(), nil)
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
	a := newFakeAgent("ok", []types.ToolCall{{ID: "call_1", Name: "time", Arguments: "{}"}})

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

	a := newFakeAgent("ok", []types.ToolCall{{ID: "call_1", Name: "view", Arguments: `{"path":"` + testFile + `"}`}})

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
	a := New(fp, reg, sess, slog.Default(), nil)
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
	got := types.FormatToolCall("time", "{}")
	if got != "time()" {
		t.Errorf("types.FormatToolCall(time, '{}') = %q, want %q", got, "time()")
	}
}

func TestFormatToolCall_noArgsEmpty(t *testing.T) {
	got := types.FormatToolCall("time", "")
	if got != "time()" {
		t.Errorf("types.FormatToolCall(time, '') = %q, want %q", got, "time()")
	}
}

func TestFormatToolCall_withArgs(t *testing.T) {
	got := types.FormatToolCall("view", `{"path":"foo/output.txt"}`)
	want := `view(path="foo/output.txt")`
	if got != want {
		t.Errorf("types.FormatToolCall(view, ...) = %q, want %q", got, want)
	}
}

func TestFormatToolCall_multipleArgs(t *testing.T) {
	got := types.FormatToolCall("list", `{"path":"foo/bar","all":false}`)
	want := `list(path="foo/bar", all=false)`
	if got != want && got != `list(all=false, path="foo/bar")` {
		t.Errorf("types.FormatToolCall(list, ...) = %q, want %q", got, want)
	}
}

func TestFormatToolCall_invalidJSON(t *testing.T) {
	got := types.FormatToolCall("view", "not json")
	if got != "view(not json)" {
		t.Errorf("types.FormatToolCall with invalid JSON should fall back to raw string, got %q", got)
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

func TestReset_clearsHistory(t *testing.T) {
	a := newFakeAgent("answer", nil)

	// Process a message to build history.
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {
	}

	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages before reset, got %d", len(h))
	}

	// Reset without kickoff.
	respCh, err := a.Reset(context.Background(), "")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}

	// Consume the marker response.
	for r := range respCh {
		if r.Role != "reset" || !r.Done {
			t.Errorf("expected reset marker, got %+v", r)
		}
	}

	h = a.History()
	if len(h) != 0 {
		t.Errorf("expected empty history after reset, got %d messages", len(h))
	}
}

func TestReset_withKickoff(t *testing.T) {
	a := newFakeAgent("kickoff answer", nil)

	// Build some history first.
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {
	}

	if len(a.History()) != 2 {
		t.Fatalf("expected 2 messages before reset, got %d", len(a.History()))
	}

	// Reset with kickoff.
	respCh, err := a.Reset(context.Background(), "kickoff message")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) == 0 {
		t.Fatal("expected responses from kickoff, got none")
	}
	if !responses[len(responses)-1].Done {
		t.Error("expected final kickoff response to have done=true")
	}

	// History should now contain only the kickoff exchange.
	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages after reset+kickoff, got %d", len(h))
	}
	if h[0].Role != "user" || h[0].Content != "kickoff message" {
		t.Errorf("history[0] = %+v, want {user, kickoff message}", h[0])
	}
	if h[1].Role != "assistant" || h[1].Content != "kickoff answer" {
		t.Errorf("history[1] = %+v, want {assistant, kickoff answer}", h[1])
	}
}

func TestReset_resetsTurnSeq(t *testing.T) {
	a := newFakeAgent("answer", nil)

	// Process two messages to increment turnSeq.
	respCh, _ := a.ProcessMessage(context.Background(), "q1")
	for range respCh {
	}
	respCh, _ = a.ProcessMessage(context.Background(), "q2")
	for range respCh {
	}

	// Reset.
	respCh, err := a.Reset(context.Background(), "")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	for range respCh {
	}

	// Process another message — turn should be 1.
	respCh, _ = a.ProcessMessage(context.Background(), "q3")
	for range respCh {
	}

	// The agent's turnSeq should have been reset (we can't directly check it,
	// but we verify the history is clean).
	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages after reset+process, got %d", len(h))
	}
}

func TestReset_withSession(t *testing.T) {
	sess, _ := session.New()
	defer sess.Close()

	a := New(&fakeProvider{callText: "answer"}, tools.NewRegistry(), sess, slog.Default(), nil)

	// Build history.
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {
	}

	h, err := sess.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(h) != 2 {
		t.Fatalf("expected 2 messages in session before reset, got %d", len(h))
	}

	// Reset: archive the old session and create a new one (simulates server behavior).
	_, err = sess.ArchiveCurrent()
	if err != nil {
		t.Fatalf("ArchiveCurrent: %v", err)
	}
	respCh, err = a.Reset(context.Background(), "")
	if err != nil {
		t.Fatalf("Reset: %v", err)
	}
	for range respCh {
	}

	// Old session should be archived; new session should be empty.
	h, err = sess.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory after reset: %v", err)
	}
	if len(h) != 0 {
		t.Errorf("expected empty active session after reset, got %d messages", len(h))
	}
}

func TestProcessMessage_bashError_providesFeedback(t *testing.T) {
	a := newFakeAgent("", []types.ToolCall{
		{ID: "call_1", Name: "bash", Arguments: `{"command":"nonexistent_command_xyz"}`},
	})

	respCh, err := a.ProcessMessage(context.Background(), "run something")
	if err != nil {
		t.Fatalf("ProcessMessage: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	// Should have: tool_call, error (red line), then text response from LLM.
	if len(responses) < 3 {
		t.Fatalf("expected at least 3 responses (tool_call + error + text), got %d", len(responses))
	}
	if responses[0].Role != "tool_call" {
		t.Errorf("first response role = %q, want %q", responses[0].Role, "tool_call")
	}
	if responses[1].Role != "error" {
		t.Errorf("second response role = %q, want %q", responses[1].Role, "error")
	}
	if !strings.Contains(responses[1].Content, "nonexistent_command_xyz") {
		t.Errorf("error response should mention the command, got %q", responses[1].Content)
	}

	// History should contain a user-role message with the error (automatic feedback).
	h := a.History()
	userMsgFound := false
	for _, m := range h {
		if m.Role == "user" && strings.Contains(m.Content, "nonexistent_command_xyz") {
			userMsgFound = true
			break
		}
	}
	if !userMsgFound {
		t.Error("expected user-role message with bash error in history (automatic feedback)")
	}
}

func TestCompact_success(t *testing.T) {
	sess, _ := session.New()
	defer sess.Close()

	a := New(&fakeProvider{callText: "You are an AI assistant."}, tools.NewRegistry(), sess, slog.Default(), nil)

	// Build history (each ProcessMessage adds user + assistant = 2 messages).
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {}

	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages before compact, got %d", len(h))
	}

	// Compact.
	respCh, err := a.Compact(context.Background())
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Role != "assistant" {
		t.Errorf("response role = %q, want %q", responses[0].Role, "assistant")
	}
	if !responses[0].Done {
		t.Error("expected done=true")
	}
	if responses[0].Content != "You are an AI assistant." {
		t.Errorf("response content = %q, want %q", responses[0].Content, "You are an AI assistant.")
	}

	// History should now contain only the summary.
	h = a.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 message after compact, got %d", len(h))
	}
	if h[0].Role != "assistant" || h[0].Content != "You are an AI assistant." {
		t.Errorf("history[0] = %+v, want {assistant, summary}", h[0])
	}

	// Session should have the summary saved.
	saved, err := sess.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(saved) != 1 {
		t.Fatalf("expected 1 message in session after compact, got %d", len(saved))
	}
	if saved[0].Role != "assistant" {
		t.Errorf("session message[0] role = %q, want %q", saved[0].Role, "assistant")
	}
}

func TestCompact_failure(t *testing.T) {
	sess, _ := session.New()
	defer sess.Close()

	a := New(&fakeProvider{callErr: fmt.Errorf("api error")}, tools.NewRegistry(), sess, slog.Default(), nil)

	// Build history.
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {}

	// Compact should fail but leave session intact.
	respCh, err := a.Compact(context.Background())
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Role != "error" {
		t.Errorf("response role = %q, want %q", responses[0].Role, "error")
	}
	if !strings.Contains(responses[0].Content, "api error") {
		t.Errorf("error content should mention the error, got %q", responses[0].Content)
	}

	// History should be unchanged (1 message: user only, no assistant on error).
	h := a.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 message (unchanged), got %d", len(h))
	}
	if h[0].Role != "user" || h[0].Content != "question" {
		t.Errorf("history[0] = %+v, want {user, question}", h[0])
	}

	// Session should be unchanged (no messages saved on provider error).
	saved, err := sess.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(saved) != 0 {
		t.Fatalf("expected 0 messages in session (none saved on error), got %d", len(saved))
	}
}

func TestCompact_emptyHistory(t *testing.T) {
	sess, _ := session.New()
	defer sess.Close()

	a := New(&fakeProvider{callText: "empty session summary"}, tools.NewRegistry(), sess, slog.Default(), nil)

	// Compact with no history.
	respCh, err := a.Compact(context.Background())
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if responses[0].Role != "assistant" || responses[0].Content != "empty session summary" {
		t.Errorf("response = %+v", responses[0])
	}
}

func TestBuildTranscript_basic(t *testing.T) {
	msgs := []types.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi there"},
	}
	got := buildTranscript(msgs)
	want := "## user\nhello\n\n## assistant\nhi there\n\n"
	if got != want {
		t.Errorf("buildTranscript() = %q, want %q", got, want)
	}
}

func TestBuildTranscript_withToolCalls(t *testing.T) {
	msgs := []types.Message{
		{Role: "user", Content: "list files"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "c1", Name: "list", Arguments: `{"path":"."}`}}},
		{Role: "tool", Content: "file1.go\nfile2.go", ToolID: "c1"},
		{Role: "assistant", Content: "done"},
	}
	got := buildTranscript(msgs)
	if !strings.Contains(got, "## user") || !strings.Contains(got, "## assistant") || !strings.Contains(got, "## tool") {
		t.Errorf("missing role headings in transcript: %q", got)
	}
	if !strings.Contains(got, "list({\"path\":\".\"})") {
		t.Errorf("missing tool call in transcript: %q", got)
	}
	if !strings.Contains(got, "file1.go") {
		t.Errorf("missing tool result in transcript: %q", got)
	}
}

func TestBuildTranscript_empty(t *testing.T) {
	got := buildTranscript(nil)
	if got != "" {
		t.Errorf("buildTranscript(nil) = %q, want empty string", got)
	}
}

func TestCompactAndReset_withKickoff(t *testing.T) {
	sess, _ := session.New()
	defer sess.Close()

	// Use a provider that can return different values for different calls.
	// callTexts is consumed in order: ProcessMessage call, Compact call, Kickoff call.
	fp := &fakeProvider{
		callTexts: []string{"dummy", "session summary", "kickoff answer"},
	}
	a := New(fp, tools.NewRegistry(), sess, slog.Default(), nil)

	// Build some history (consumes first callTexts entry).
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {}

	// CompactAndReset with kickoff.
	respCh, err := a.CompactAndReset(context.Background(), "kickoff message")
	if err != nil {
		t.Fatalf("CompactAndReset: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses (summary + kickoff answer), got %d", len(responses))
	}
	if responses[0].Role != "assistant" || responses[0].Content != "session summary" {
		t.Errorf("first response = %+v, want {assistant, session summary}", responses[0])
	}
	if responses[0].Done {
		t.Error("expected done=false for compact summary")
	}
	if responses[1].Role != "assistant" || responses[1].Content != "kickoff answer" {
		t.Errorf("second response = %+v, want {assistant, kickoff answer}", responses[1])
	}
	if !responses[1].Done {
		t.Error("expected done=true for final response")
	}

	// History should contain summary + kickoff exchange.
	h := a.History()
	if len(h) != 3 {
		t.Fatalf("expected 3 messages after compact+reset+kickoff, got %d", len(h))
	}
	if h[0].Role != "assistant" || h[0].Content != "session summary" {
		t.Errorf("history[0] = %+v, want {assistant, session summary}", h[0])
	}
	if h[1].Role != "user" || h[1].Content != "kickoff message" {
		t.Errorf("history[1] = %+v, want {user, kickoff message}", h[1])
	}
	if h[2].Role != "assistant" || h[2].Content != "kickoff answer" {
		t.Errorf("history[2] = %+v, want {assistant, kickoff answer}", h[2])
	}
}

func TestCompactAndReset_withoutKickoff(t *testing.T) {
	sess, _ := session.New()
	defer sess.Close()

	a := New(&fakeProvider{callText: "session summary"}, tools.NewRegistry(), sess, slog.Default(), nil)

	// Build some history.
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {}

	// CompactAndReset without kickoff.
	respCh, err := a.CompactAndReset(context.Background(), "")
	if err != nil {
		t.Fatalf("CompactAndReset: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 2 {
		t.Fatalf("expected 2 responses (summary + reset marker), got %d", len(responses))
	}
	if responses[0].Role != "assistant" || responses[0].Content != "session summary" {
		t.Errorf("first response = %+v, want {assistant, session summary}", responses[0])
	}
	if responses[0].Done {
		t.Error("expected done=false for compact summary")
	}
	if responses[1].Role != "reset" || !responses[1].Done {
		t.Errorf("second response = %+v, want {reset, done=true}", responses[1])
	}

	// History should contain only the summary.
	h := a.History()
	if len(h) != 1 {
		t.Fatalf("expected 1 message after compact+reset, got %d", len(h))
	}
	if h[0].Role != "assistant" || h[0].Content != "session summary" {
		t.Errorf("history[0] = %+v, want {assistant, session summary}", h[0])
	}
}

func TestCompactAndReset_providerError(t *testing.T) {
	sess, _ := session.New()
	defer sess.Close()

	// Provider succeeds on ProcessMessage (call 1), errors on CompactAndReset (call 2).
	fp := &fakeProvider{
		callText:     "answer",
		callErr:      context.Canceled,
		callErrAfter: 1, // error after 1 successful call
	}
	a := New(fp, tools.NewRegistry(), sess, slog.Default(), nil)

	// Build some history.
	respCh, _ := a.ProcessMessage(context.Background(), "question")
	for range respCh {}

	// CompactAndReset with provider error.
	respCh, err := a.CompactAndReset(context.Background(), "kickoff message")
	if err != nil {
		t.Fatalf("CompactAndReset: %v", err)
	}

	var responses []*types.Response
	for r := range respCh {
		responses = append(responses, r)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response (error), got %d", len(responses))
	}
	if responses[0].Role != "error" {
		t.Errorf("response role = %q, want %q", responses[0].Role, "error")
	}
	if !strings.Contains(responses[0].Content, "context canceled") {
		t.Errorf("expected error content, got %q", responses[0].Content)
	}
	if !responses[0].Done {
		t.Error("expected done=true for error response")
	}

	// History should be unchanged.
	h := a.History()
	if len(h) != 2 {
		t.Fatalf("expected 2 messages (unchanged), got %d", len(h))
	}
}
