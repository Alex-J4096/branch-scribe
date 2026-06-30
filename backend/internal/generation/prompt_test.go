package generation

import (
	"context"
	"testing"
)

func TestRenderUserPromptCanSkipWritingOperation(t *testing.T) {
	disabled := false
	template := &PromptTemplate{ID: "template", TemplateText: "wrapped {{user_instruction}}"}
	prompt, templateID := renderUserPrompt(
		GenerateOnceRequest{
			TaskType:            "continue",
			UserInstruction:     "只继续这一句话",
			ApplyPromptTemplate: &disabled,
		},
		BlockContext{},
		map[string]string{"current_block": "不应进入消息"},
		template,
	)
	if prompt != "只继续这一句话" {
		t.Fatalf("prompt = %q, want raw user instruction", prompt)
	}
	if templateID != nil {
		t.Fatalf("template id = %v, want nil", templateID)
	}
}

func TestBuildContextSkipsContextAssemblyWhenWritingOperationDisabled(t *testing.T) {
	disabled := false
	preview, err := (&Handler{}).buildContext(
		context.Background(),
		GenerateOnceRequest{
			UserInstruction:     "继续讨论人物动机",
			ApplyPromptTemplate: &disabled,
		},
		BlockContext{Content: "不应读取或发送的当前正文"},
		nil,
		4000,
	)
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if preview.UserPrompt != "继续讨论人物动机" {
		t.Fatalf("user prompt = %q", preview.UserPrompt)
	}
	if len(preview.Items) != 0 || preview.PromptTemplateID != nil {
		t.Fatalf("disabled operation still assembled context: %#v", preview)
	}
}
