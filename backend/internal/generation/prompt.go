package generation

import (
	"encoding/json"
	"html"
	"regexp"
	"strings"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

type promptSnapshot struct {
	ProjectID        string  `json:"project_id"`
	BlockID          string  `json:"block_id"`
	TaskType         string  `json:"task_type"`
	CurrentBlock     string  `json:"current_block"`
	SelectedText     string  `json:"selected_text"`
	UserInstruction  string  `json:"user_instruction"`
	Prompt           string  `json:"prompt"`
	PromptTemplateID *string `json:"prompt_template_id"`
}

func renderPrompt(req GenerateOnceRequest, blockContext BlockContext, template *PromptTemplate) (string, json.RawMessage) {
	currentBlock := normalizeBlockContent(blockContext.Content, blockContext.ContentFormat)
	templateText := defaultPromptTemplate(req.TaskType)
	var templateID *string
	if template != nil {
		templateText = template.TemplateText
		templateID = &template.ID
	}

	projectDescription := ""
	if blockContext.ProjectDescription != nil {
		projectDescription = *blockContext.ProjectDescription
	}
	blockTitle := ""
	if blockContext.BlockTitle != nil {
		blockTitle = *blockContext.BlockTitle
	}

	prompt := strings.NewReplacer(
		"{{project_description}}", projectDescription,
		"{{current_block_title}}", blockTitle,
		"{{current_block}}", currentBlock,
		"{{selected_text}}", req.SelectedText,
		"{{user_instruction}}", req.UserInstruction,
	).Replace(templateText)

	snapshot, _ := json.Marshal(promptSnapshot{
		ProjectID:        req.ProjectID,
		BlockID:          req.BlockID,
		TaskType:         req.TaskType,
		CurrentBlock:     currentBlock,
		SelectedText:     req.SelectedText,
		UserInstruction:  req.UserInstruction,
		Prompt:           prompt,
		PromptTemplateID: templateID,
	})
	return prompt, snapshot
}

func normalizeBlockContent(content string, format string) string {
	if format == "html" {
		content = htmlTagPattern.ReplaceAllString(content, " ")
		content = html.UnescapeString(content)
	}
	return strings.Join(strings.Fields(content), " ")
}

func defaultPromptTemplate(taskType string) string {
	switch taskType {
	case "free_write":
		return "你是小说写作助手。请完全根据用户指令生成正文，不要依赖当前 block 正文。只输出生成后的正文。\n\n项目简介：\n{{project_description}}\n\n用户指令：\n{{user_instruction}}"
	case "continue":
		return "你是小说写作助手。请基于当前片段继续写作，保持人物、语气和叙事连贯。\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "rewrite_block":
		return "你是小说写作助手。请根据用户指令改写当前片段，只输出改写后的正文。\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "rewrite_selection":
		return "你是小说写作助手。请在理解当前片段的基础上改写选中文本，只输出改写后的选中文本。\n\n当前片段：\n{{current_block}}\n\n选中文本：\n{{selected_text}}\n\n用户指令：\n{{user_instruction}}"
	case "expand":
		return "你是小说写作助手。请扩写当前片段，补充细节、动作和感官描写，只输出扩写后的正文。\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "condense":
		return "你是小说写作助手。请压缩当前片段，保留关键情节和风格，只输出压缩后的正文。\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "polish":
		return "你是小说写作助手。请润色当前片段，提升表达和节奏，只输出润色后的正文。\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	default:
		return "你是小说写作助手。请根据当前片段和用户指令完成写作任务。\n\n当前片段：\n{{current_block}}\n\n选中文本：\n{{selected_text}}\n\n用户指令：\n{{user_instruction}}"
	}
}
