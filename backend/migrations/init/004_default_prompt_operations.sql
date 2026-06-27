CREATE OR REPLACE FUNCTION seed_default_prompt_operations(target_project_id UUID)
RETURNS void
LANGUAGE sql
AS $$
    INSERT INTO prompt_templates (
        project_id,
        name,
        task_type,
        template_text,
        version,
        is_default,
        metadata
    )
    SELECT
        target_project_id,
        operation.name,
        operation.task_type,
        operation.template_text,
        1,
        true,
        jsonb_build_object('built_in', true)
    FROM (
        VALUES
            ('自由生成', 'free_write', E'请完全根据用户指令生成正文，不要依赖当前 block 正文。必须遵守硬设定，并参考相关记忆。只输出生成后的正文。\n\n项目简介：\n{{project_description}}\n\n硬设定：\n{{canon_facts}}\n\n相关记忆：\n{{memory_chunks}}\n\n用户指令：\n{{user_instruction}}'),
            ('续写', 'continue', E'请基于当前片段继续写作，保持人物、语气和叙事连贯，必须遵守硬设定。\n\n硬设定：\n{{canon_facts}}\n\n分支摘要：\n{{branch_summary}}\n\n章节摘要：\n{{chapter_summary}}\n\n最近正文：\n{{recent_blocks}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
            ('改写', 'rewrite_block', E'请根据用户指令改写当前片段，必须遵守硬设定，只输出改写后的正文。\n\n硬设定：\n{{canon_facts}}\n\n章节摘要：\n{{chapter_summary}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
            ('局部改写', 'rewrite_selection', E'请在理解当前片段、前后文和硬设定的基础上改写选中文本，只输出改写后的选中文本。\n\n硬设定：\n{{canon_facts}}\n\n章节摘要：\n{{chapter_summary}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n选中文本：\n{{selected_text}}\n\n用户指令：\n{{user_instruction}}'),
            ('扩写', 'expand', E'请扩写当前片段，补充细节、动作和感官描写，必须遵守硬设定，只输出扩写后的正文。\n\n硬设定：\n{{canon_facts}}\n\n最近正文：\n{{recent_blocks}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
            ('缩写', 'condense', E'请压缩当前片段，保留关键情节、风格和硬设定，只输出压缩后的正文。\n\n硬设定：\n{{canon_facts}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}'),
            ('润色', 'polish', E'请润色当前片段，提升表达和节奏，必须遵守硬设定，只输出润色后的正文。\n\n硬设定：\n{{canon_facts}}\n\n相关记忆：\n{{memory_chunks}}\n\n当前片段：\n{{current_block}}\n\n用户指令：\n{{user_instruction}}')
    ) AS operation(name, task_type, template_text)
    WHERE NOT EXISTS (
        SELECT 1
        FROM prompt_templates existing
        WHERE existing.project_id = target_project_id
          AND existing.task_type = operation.task_type
    );
$$;

CREATE OR REPLACE FUNCTION seed_default_prompt_operations_for_project()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM seed_default_prompt_operations(NEW.id);
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS seed_project_prompt_operations ON projects;
CREATE TRIGGER seed_project_prompt_operations
    AFTER INSERT ON projects
    FOR EACH ROW
    EXECUTE FUNCTION seed_default_prompt_operations_for_project();

CREATE TABLE IF NOT EXISTS app_migrations (
    name TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM app_migrations WHERE name = 'default_prompt_operations_v1'
    ) THEN
        PERFORM seed_default_prompt_operations(id) FROM projects;
        INSERT INTO app_migrations (name) VALUES ('default_prompt_operations_v1');
    END IF;
END;
$$;
