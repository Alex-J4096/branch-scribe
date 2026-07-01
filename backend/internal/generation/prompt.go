package generation

import (
	"html"
	"regexp"
	"strings"
)

var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

type promptBlockVariable struct {
	Placeholder string
	Tag         string
	Value       string
}

func renderUserPrompt(req GenerateOnceRequest, blockContext BlockContext, contextText map[string]string, template *PromptTemplate) (string, *string) {
	if !req.shouldApplyPromptTemplate() {
		return conversationUserContent(req), nil
	}
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

	prompt := templateText
	for _, variable := range []promptBlockVariable{
		{Placeholder: "{{project_description}}", Tag: "项目简介", Value: projectDescription},
		{Placeholder: "{{current_block_title}}", Tag: "当前片段标题", Value: blockTitle},
		{Placeholder: "{{current_block}}", Tag: "当前片段", Value: contextText["current_block"]},
		{Placeholder: "{{canon_facts}}", Tag: "硬设定", Value: contextText["canon_facts"]},
		{Placeholder: "{{recent_blocks}}", Tag: "最近正文", Value: contextText["recent_blocks"]},
		{Placeholder: "{{branch_summary}}", Tag: "分支摘要", Value: contextText["branch_summary"]},
		{Placeholder: "{{chapter_summary}}", Tag: "章节摘要", Value: contextText["chapter_summary"]},
		{Placeholder: "{{memory_chunks}}", Tag: "相关记忆", Value: contextText["memory_chunks"]},
		{Placeholder: "{{selected_text}}", Tag: "选中文本", Value: req.SelectedText},
		{Placeholder: "{{user_instruction}}", Tag: "用户指令", Value: req.UserInstruction},
	} {
		prompt = renderPromptBlockVariable(prompt, variable)
	}

	return prompt, templateID
}

func renderPromptBlockVariable(templateText string, variable promptBlockVariable) string {
	block := "<" + variable.Tag + ">\n" + variable.Value + "\n</" + variable.Tag + ">"
	for _, taggedPlaceholder := range []string{
		"<" + variable.Tag + ">" + variable.Placeholder + "</" + variable.Tag + ">",
		"<" + variable.Tag + ">\n" + variable.Placeholder + "\n</" + variable.Tag + ">",
		"<" + variable.Tag + ">\r\n" + variable.Placeholder + "\r\n</" + variable.Tag + ">",
		variable.Tag + "：\n" + variable.Placeholder,
		variable.Tag + "：\r\n" + variable.Placeholder,
		variable.Tag + ":\n" + variable.Placeholder,
		variable.Tag + ":\r\n" + variable.Placeholder,
	} {
		templateText = strings.ReplaceAll(templateText, taggedPlaceholder, block)
	}
	return strings.ReplaceAll(templateText, variable.Placeholder, block)
}

func defaultSystemPrompt() string {
	return "你是 BranchScribe 的小说创作助手。严格遵守已给出的设定、上下文和用户指令；只输出可直接放入小说正文的内容，除非用户明确要求解释。"
}

func renderCanonFacts(facts []CanonFact) string {
	if len(facts) == 0 {
		return "无"
	}

	lines := make([]string, 0, len(facts))
	for _, fact := range facts {
		parts := []string{fact.Type, fact.Name}
		if len(fact.Aliases) > 0 {
			parts = append(parts, "别名："+strings.Join(fact.Aliases, "、"))
		}
		if fact.Description != nil && strings.TrimSpace(*fact.Description) != "" {
			parts = append(parts, strings.TrimSpace(*fact.Description))
		}
		lines = append(lines, "- "+strings.Join(parts, "｜"))
	}
	return strings.Join(lines, "\n")
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
	case "compare_revisions":
		return "请为当前片段生成一个可独立比较的候选版本。保持核心设定一致，但在情节选择、叙事角度、节奏或语言表达上形成清晰差异。只输出完整候选正文。\n\n硬设定：\n{{canon_facts}}\n\n分支摘要：\n{{branch_summary}}\n\n最近正文：\n{{recent_blocks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "free_write":
		return "请完全根据用户指令生成正文，不要依赖当前 block 正文。必须遵守硬设定，并参考相关记忆。只输出生成后的正文。\n\n项目简介：\n{{project_description}}\n\n硬设定：\n{{canon_facts}}\n\n相关记忆：\n{{memory_chunks}}\n\n用户指令：\n{{user_instruction}}"
	case "continue":
		return "请基于当前片段继续写作，保持人物、语气和叙事连贯，必须遵守硬设定。\n\n硬设定：\n{{canon_facts}}\n\n分支摘要：\n{{branch_summary}}\n\n章节摘要：\n{{chapter_summary}}\n\n最近正文：\n{{recent_blocks}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "rewrite_block":
		return "请根据用户指令改写当前片段，必须遵守硬设定，只输出改写后的正文。\n\n硬设定：\n{{canon_facts}}\n\n章节摘要：\n{{chapter_summary}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "rewrite_selection":
		return "请在理解当前片段、前后文和硬设定的基础上改写选中文本，只输出改写后的选中文本。\n\n硬设定：\n{{canon_facts}}\n\n章节摘要：\n{{chapter_summary}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n选中文本：\n{{selected_text}}\n\n用户指令：\n{{user_instruction}}"
	case "expand":
		return "请扩写当前片段，补充细节、动作和感官描写，必须遵守硬设定，只输出扩写后的正文。\n\n硬设定：\n{{canon_facts}}\n\n最近正文：\n{{recent_blocks}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "condense":
		return "请压缩当前片段，保留关键情节、风格和硬设定，只输出压缩后的正文。\n\n硬设定：\n{{canon_facts}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	case "polish":
		return "请润色当前片段，提升表达和节奏，必须遵守硬设定，只输出润色后的正文。\n\n硬设定：\n{{canon_facts}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}"
	default:
		return "请根据当前片段、硬设定、相关记忆和用户指令完成写作任务。\n\n硬设定：\n{{canon_facts}}\n\n最近正文：\n{{recent_blocks}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n选中文本：\n{{selected_text}}\n\n用户指令：\n{{user_instruction}}"
	}
}
