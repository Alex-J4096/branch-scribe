<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, MessageSquareText, Plus, Save, Trash2 } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'
import type { PromptTemplate, PromptTemplateInput } from '@/api/types'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const projectId = computed(() => String(route.params.projectId))
const requestedTask = computed(() => typeof route.query.task === 'string' ? route.query.task : '')
const selectedId = ref<string | null>(null)
const form = reactive<PromptTemplateInput>({
  name: '',
  task_type: 'continue',
  template_text: '',
  is_default: false,
})

const taskGroups = [
  { label: '写作', tasks: ['free_write', 'continue', 'rewrite_block', 'rewrite_selection', 'expand', 'condense', 'polish'] },
  { label: '摘要', tasks: ['block_summary', 'chapter_summary', 'branch_summary'] },
]
const taskLabels: Record<string, string> = {
  free_write: '自由生成', continue: '续写', rewrite_block: '改写', rewrite_selection: '局部改写',
  expand: '扩写', condense: '缩写', polish: '润色', block_summary: 'Block 摘要',
  chapter_summary: '章节摘要', branch_summary: '分支摘要',
}
const variables: Record<string, string[]> = {
  block_summary: ['{{title}}', '{{target_type}}', '{{content}}'],
  chapter_summary: ['{{title}}', '{{target_type}}', '{{content}}'],
  branch_summary: ['{{title}}', '{{target_type}}', '{{content}}'],
}
const writingVariables = ['{{project_description}}', '{{canon_facts}}', '{{branch_summary}}', '{{chapter_summary}}', '{{recent_blocks}}', '{{memory_chunks}}', '{{current_block}}', '{{selected_text}}', '{{user_instruction}}']
const availableVariables = computed(() => variables[form.task_type] ?? writingVariables)

const templatesQuery = useQuery({
  queryKey: computed(() => ['prompt-templates', projectId.value]),
  queryFn: () => api.listPromptTemplates(projectId.value),
})
const templates = computed(() => templatesQuery.data.value ?? [])
const groupedTemplates = computed(() => taskGroups.map((group) => ({
  ...group,
  templates: templates.value.filter((template) => group.tasks.includes(template.task_type)),
})))

watch(templates, (items) => {
  if (selectedId.value) return
  const preferred = items.find((item) => item.task_type === requestedTask.value)
  if (preferred) {
    selectTemplate(preferred)
  } else if (requestedTask.value && taskLabels[requestedTask.value]) {
    resetForm(requestedTask.value)
  } else if (items[0]) {
    selectTemplate(items[0])
  }
}, { immediate: true })

const saveTemplate = useMutation({
  mutationFn: () => selectedId.value
    ? api.updatePromptTemplate(selectedId.value, { ...form })
    : api.createPromptTemplate(projectId.value, { ...form }),
  onSuccess: async (saved) => {
    selectedId.value = saved.id
    await queryClient.invalidateQueries({ queryKey: ['prompt-templates', projectId.value] })
  },
})
const deleteTemplate = useMutation({
  mutationFn: (id: string) => api.deletePromptTemplate(id),
  onSuccess: async () => {
    selectedId.value = null
    resetForm()
    await queryClient.invalidateQueries({ queryKey: ['prompt-templates', projectId.value] })
  },
})

function selectTemplate(template: PromptTemplate) {
  selectedId.value = template.id
  form.name = template.name
  form.task_type = template.task_type
  form.template_text = template.template_text
  form.is_default = template.is_default
}
function resetForm(taskType = requestedTask.value || 'continue') {
  selectedId.value = null
  form.name = ''
  form.task_type = taskType
  form.template_text = ''
  form.is_default = false
}

function goBack() {
  if (typeof route.query.from === 'string' && route.query.from.startsWith('/')) {
    void router.push(route.query.from)
    return
  }
  void router.push({ name: 'workspace', params: { projectId: projectId.value } })
}
</script>

<template>
  <main class="settings-page">
    <header class="workspace__topbar">
      <button class="icon-button" type="button" title="返回工作台" @click="goBack">
        <ArrowLeft :size="18" aria-hidden="true" />
      </button>
      <div class="workspace__title">
        <strong>Prompt 管理</strong>
        <span>写作操作与摘要模板</span>
      </div>
      <button class="button" type="button" @click="resetForm()">
        <Plus :size="16" aria-hidden="true" />新建 Prompt
      </button>
    </header>

    <section class="settings-page__body">
      <aside class="settings-list">
        <template v-for="group in groupedTemplates" :key="group.label">
          <div class="settings-list__header">
            <span>{{ group.label }}</span>
            <small>{{ group.templates.length }}</small>
          </div>
          <div v-if="group.templates.length === 0" class="empty-state empty-state--compact">暂无{{ group.label }}模板</div>
          <button
            v-for="template in group.templates" :key="template.id"
            class="settings-list__item" :class="{ 'is-active': selectedId === template.id }"
            type="button" @click="selectTemplate(template)"
          >
            <strong>{{ template.name }}</strong>
            <small>{{ taskLabels[template.task_type] ?? template.task_type }}{{ template.is_default ? ' · 默认' : '' }}</small>
          </button>
        </template>
      </aside>

      <form class="settings-form prompt-settings-form" @submit.prevent="saveTemplate.mutate()">
        <div class="settings-form__header">
          <div>
            <h1>{{ selectedId ? form.name || '编辑 Prompt' : '新建 Prompt' }}</h1>
            <p>{{ taskLabels[form.task_type] ?? form.task_type }} · {{ selectedId ? '编辑现有模板' : '创建项目模板' }}</p>
          </div>
          <MessageSquareText :size="22" aria-hidden="true" />
        </div>

        <section class="settings-group">
          <div class="settings-form__grid">
            <label><span>名称</span><input v-model="form.name" required /></label>
            <label><span>操作类型</span>
              <select v-model="form.task_type" required>
                <optgroup v-for="group in taskGroups" :key="group.label" :label="group.label">
                  <option v-for="task in group.tasks" :key="task" :value="task">{{ taskLabels[task] }}</option>
                </optgroup>
              </select>
            </label>
            <label class="settings-form__wide">
              <span>Prompt 模板</span>
              <textarea v-model="form.template_text" class="prompt-template-editor" spellcheck="false" required />
            </label>
            <p class="settings-form__hint settings-form__wide">可用变量：{{ availableVariables.join('、') }}</p>
            <label class="settings-form__check settings-form__wide">
              <input v-model="form.is_default" type="checkbox" />
              <span>设为该操作的默认 Prompt</span>
            </label>
          </div>
        </section>
        <footer class="settings-form__actions">
          <button class="button button--primary" type="submit" :disabled="saveTemplate.isPending.value">
            <Save :size="16" aria-hidden="true" />{{ saveTemplate.isPending.value ? '保存中' : '保存' }}
          </button>
          <button
            v-if="selectedId"
            class="button button--danger"
            type="button"
            :disabled="deleteTemplate.isPending.value"
            @click="deleteTemplate.mutate(selectedId)"
          >
            <Trash2 :size="16" aria-hidden="true" />删除
          </button>
        </footer>
      </form>
    </section>
  </main>
</template>
