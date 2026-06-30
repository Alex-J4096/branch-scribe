INSERT INTO prompt_templates (
    project_id, name, task_type, template_text, version, is_default, metadata
)
SELECT project.id, operation.name, operation.task_type, operation.template_text, 1, true,
    jsonb_build_object('built_in', true, 'category', 'summary')
FROM projects project
CROSS JOIN (
    VALUES
        ('Block 摘要', 'block_summary', E'请准确、简洁地概括以下小说片段，保留关键人物、事件、因果、地点与未解决冲突，不添加原文没有的信息。只输出摘要正文。\n\n标题：{{title}}\n\n正文：\n{{content}}'),
        ('章节摘要', 'chapter_summary', E'请概括以下章节内容，保留情节推进、人物状态变化、重要设定与未解决冲突。只输出摘要正文。\n\n标题：{{title}}\n\n章节内容：\n{{content}}'),
        ('分支摘要', 'branch_summary', E'请将以下故事分支整理为连贯摘要，保留关键事件的先后与因果、人物状态变化、重要设定及未解决线索。输入可能是完整 Block 正文，也可能是用户选择的 Block 摘要。只输出摘要正文。\n\n分支：{{title}}\n\n内容：\n{{content}}')
) AS operation(name, task_type, template_text)
WHERE NOT EXISTS (
    SELECT 1
    FROM prompt_templates existing
    WHERE existing.project_id = project.id
      AND existing.task_type = operation.task_type
);
