<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMutation, useQuery } from '@tanstack/vue-query'
import { ArrowLeft, Archive, Download, FileText, Upload } from 'lucide-vue-next'
import { useRoute, useRouter } from 'vue-router'

import { api } from '@/api/client'

const route = useRoute()
const router = useRouter()
const projectId = computed(() => String(route.params.projectId))
const exportScope = ref<'branch' | 'chapter'>('branch')
const exportFormat = ref<'markdown'>('markdown')
const selectedBranchId = ref('')
const selectedChapterId = ref('')
const exportMessage = ref('')
const backupMessage = ref('')
const importMessage = ref('')
const importFile = ref<File | null>(null)

const projectQuery = useQuery({
  queryKey: computed(() => ['project', projectId.value]),
  queryFn: () => api.getProject(projectId.value),
})
const branchesQuery = useQuery({
  queryKey: computed(() => ['branches', projectId.value]),
  queryFn: () => api.listBranches(projectId.value),
})
const blocksQuery = useQuery({
  queryKey: computed(() => ['blocks', projectId.value]),
  queryFn: () => api.listBlocks(projectId.value),
})
const branches = computed(() => branchesQuery.data.value ?? [])
const chapters = computed(() => (blocksQuery.data.value ?? []).filter((block) => block.type === 'chapter'))

watch(branches, (items) => {
  if (!items.some((item) => item.id === selectedBranchId.value)) selectedBranchId.value = items[0]?.id ?? ''
}, { immediate: true })
watch(chapters, (items) => {
  if (!items.some((item) => item.id === selectedChapterId.value)) selectedChapterId.value = items[0]?.id ?? ''
}, { immediate: true })

const exportMarkdown = useMutation({
  mutationFn: () => api.downloadMarkdownExport(projectId.value, exportScope.value === 'branch'
    ? { branchId: selectedBranchId.value }
    : { chapterId: selectedChapterId.value }),
  onSuccess: (blob) => {
    const targetName = exportScope.value === 'branch'
      ? branches.value.find((item) => item.id === selectedBranchId.value)?.name
      : chapters.value.find((item) => item.id === selectedChapterId.value)?.title
    saveBlob(blob, `${projectQuery.data.value?.name ?? 'branchscribe'}-${targetName ?? exportScope.value}.md`)
    exportMessage.value = 'Markdown 已开始下载'
  },
  onError: (error) => { exportMessage.value = errorMessage(error, '导出失败') },
})

const downloadBackup = useMutation({
  mutationFn: () => api.downloadProjectBackup(projectId.value),
  onSuccess: (blob) => {
    saveBlob(blob, `branchscribe-${safeFilename(projectQuery.data.value?.name ?? projectId.value)}.json`)
    backupMessage.value = '项目备份已开始下载。备份不包含 API Key 和向量索引。'
  },
  onError: (error) => { backupMessage.value = errorMessage(error, '备份失败') },
})

const importBackup = useMutation({
  mutationFn: async () => {
    if (!importFile.value) throw new Error('请先选择 JSON 备份文件')
    const text = await importFile.value.text()
    let backup: unknown
    try {
      backup = JSON.parse(text)
    } catch {
      throw new Error('文件不是有效的 JSON')
    }
    return api.importProjectBackup(backup)
  },
  onSuccess: async (result) => {
    importMessage.value = '项目恢复成功，正在打开…'
    await router.push({ name: 'workspace', params: { projectId: result.project_id } })
  },
  onError: (error) => { importMessage.value = errorMessage(error, '导入失败') },
})

function handleImportFile(event: Event) {
  importFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
  importMessage.value = ''
}

function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = safeFilename(filename)
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

function safeFilename(value: string) {
  return value.replace(/[\\/:*?"<>|]+/g, '-').trim() || 'branchscribe-export'
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}
</script>

<template>
  <main class="manager-page transfer-page">
    <header class="manager-page__topbar">
      <button class="icon-button" type="button" title="返回工作台" @click="router.push({ name: 'workspace', params: { projectId } })">
        <ArrowLeft :size="18" aria-hidden="true" />
      </button>
      <div>
        <h1>导出与备份</h1>
        <p>{{ projectQuery.data.value?.name ?? '项目' }}</p>
      </div>
    </header>

    <section class="transfer-grid">
      <section class="transfer-card">
        <header><FileText :size="18" /><div><h2>导出正文</h2><p>将故事内容导出为可独立阅读的 Markdown 文件。</p></div></header>
        <div class="manager-form__row">
          <label class="field-label">
            <span>导出范围</span>
            <select v-model="exportScope">
              <option value="branch">故事分支</option>
              <option value="chapter">章节</option>
            </select>
          </label>
          <label class="field-label">
            <span>格式</span>
            <select v-model="exportFormat">
              <option value="markdown">Markdown (.md)</option>
            </select>
          </label>
        </div>
        <label v-if="exportScope === 'branch'" class="field-label">
          <span>选择 Branch</span>
          <select v-model="selectedBranchId">
            <option v-for="branch in branches" :key="branch.id" :value="branch.id">{{ branch.name }}</option>
          </select>
        </label>
        <label v-else class="field-label">
          <span>选择 Chapter</span>
          <select v-model="selectedChapterId">
            <option v-for="chapter in chapters" :key="chapter.id" :value="chapter.id">{{ chapter.title || '未命名章节' }}</option>
          </select>
        </label>
        <p v-if="exportScope === 'chapter' && !chapters.length" class="transfer-note">项目中还没有 Chapter 类型的 Block。</p>
        <button class="button button--primary" type="button" :disabled="exportMarkdown.isPending.value || (exportScope === 'branch' ? !selectedBranchId : !selectedChapterId)" @click="exportMarkdown.mutate()">
          <Download :size="16" />{{ exportMarkdown.isPending.value ? '正在导出' : '下载 Markdown' }}
        </button>
        <p v-if="exportMessage" class="transfer-message">{{ exportMessage }}</p>
      </section>

      <section class="transfer-card">
        <header><Archive :size="18" /><div><h2>项目备份</h2><p>下载可恢复项目结构与内容的 JSON 文件。</p></div></header>
        <p class="transfer-note">包含分支、正文版本、设定、Memory、摘要、时间线和对话记录；不包含 API Key 与 embedding。</p>
        <button class="button button--primary" type="button" :disabled="downloadBackup.isPending.value" @click="downloadBackup.mutate()">
          <Download :size="16" />{{ downloadBackup.isPending.value ? '正在打包' : '下载 JSON 备份' }}
        </button>
        <p v-if="backupMessage" class="transfer-message">{{ backupMessage }}</p>
      </section>

      <section class="transfer-card">
        <header><Upload :size="18" /><div><h2>恢复项目</h2><p>从 BranchScribe JSON 备份恢复一个项目。</p></div></header>
        <label class="field-label transfer-file">
          <span>备份文件</span>
          <input type="file" accept="application/json,.json" @change="handleImportFile" />
        </label>
        <p class="transfer-note">恢复会保留原 Project UUID。如果该项目仍存在，系统会拒绝覆盖。</p>
        <button class="button button--primary" type="button" :disabled="!importFile || importBackup.isPending.value" @click="importBackup.mutate()">
          <Upload :size="16" />{{ importBackup.isPending.value ? '正在恢复' : '导入项目备份' }}
        </button>
        <p v-if="importMessage" class="transfer-message">{{ importMessage }}</p>
      </section>
    </section>
  </main>
</template>
