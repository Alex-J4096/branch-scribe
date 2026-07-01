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

func TestRenderUserPromptWrapsTemplateVariablesInChineseTags(t *testing.T) {
	enabled := true
	template := &PromptTemplate{
		ID:           "template",
		TemplateText: "写作要求\n\n硬设定：\n{{canon_facts}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}",
	}
	prompt, _ := renderUserPrompt(
		GenerateOnceRequest{
			UserInstruction:     "继续写下去",
			ApplyPromptTemplate: &enabled,
		},
		BlockContext{},
		map[string]string{
			"canon_facts":   "角色不能飞",
			"current_block": "他站在山顶。",
		},
		template,
	)
	want := "写作要求\n\n<硬设定>\n角色不能飞\n</硬设定>\n\n<当前片段>\n他站在山顶。\n</当前片段>\n\n<用户指令>\n继续写下去\n</用户指令>"
	if prompt != want {
		t.Fatalf("tagged prompt = %q, want %q", prompt, want)
	}
}

func TestRenderUserPromptDoesNotNestExistingTags(t *testing.T) {
	enabled := true
	template := &PromptTemplate{
		ID:           "template",
		TemplateText: "<用户指令>\n{{user_instruction}}\n</用户指令>",
	}
	prompt, _ := renderUserPrompt(
		GenerateOnceRequest{UserInstruction: "继续", ApplyPromptTemplate: &enabled},
		BlockContext{},
		nil,
		template,
	)
	if prompt != "<用户指令>\n继续\n</用户指令>" {
		t.Fatalf("prompt = %q", prompt)
	}
}
