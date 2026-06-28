<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { BookOpen, Database, Plus, Settings, Trash2, Upload, X } from 'lucide-vue-next'
import { useRouter } from 'vue-router'

import { api } from '@/api/client'

const router = useRouter()
const queryClient = useQueryClient()
const name = ref('')
const description = ref('')
const isCreateDialogOpen = ref(false)
const importMessage = ref('')
const importInput = ref<HTMLInputElement | null>(null)

const projectsQuery = useQuery({
  queryKey: ['projects'],
  queryFn: api.listProjects,
})

const projects = computed(() => projectsQuery.data.value ?? [])

const createProject = useMutation({
  mutationFn: api.createProject,
  onSuccess: async (project) => {
    name.value = ''
    description.value = ''
    isCreateDialogOpen.value = false
    await queryClient.invalidateQueries({ queryKey: ['projects'] })
    await router.push({ name: 'workspace', params: { projectId: project.id } })
  },
})

const deleteProject = useMutation({
  mutationFn: api.deleteProject,
  onSuccess: () => queryClient.invalidateQueries({ queryKey: ['projects'] }),
})

const importProject = useMutation({
  mutationFn: async (file: File) => {
    let backup: unknown
    try {
      backup = JSON.parse(await file.text())
    } catch {
      throw new Error('文件不是有效的 JSON')
    }
    return api.importProjectBackup(backup)
  },
  onSuccess: async (result) => {
    importMessage.value = ''
    await queryClient.invalidateQueries({ queryKey: ['projects'] })
    await router.push({ name: 'workspace', params: { projectId: result.project_id } })
  },
  onError: (error) => {
    importMessage.value = error instanceof Error ? error.message : '项目导入失败'
  },
})

function submitProject() {
  const trimmedName = name.value.trim()
  if (!trimmedName) return
  createProject.mutate({
    name: trimmedName,
    description: description.value.trim() || undefined,
  })
}

function handleImport(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) importProject.mutate(file)
  input.value = ''
}
</script>

<template>
  <main class="project-list">
    <section class="project-list__main">
      <div class="project-list__header">
        <div>
          <h1>BranchScribe</h1>
          <p>项目</p>
        </div>
        <div class="project-list__actions">
          <Database :size="28" aria-hidden="true" />
          <input ref="importInput" class="visually-hidden" type="file" accept="application/json,.json" @change="handleImport" />
          <button class="button" type="button" @click="router.push({ name: 'model-profiles' })">
            <Settings :size="17" aria-hidden="true" />
            全局模型
          </button>
          <button class="button" type="button" :disabled="importProject.isPending.value" @click="importInput?.click()">
            <Upload :size="17" aria-hidden="true" />
            {{ importProject.isPending.value ? '导入中' : '导入备份' }}
          </button>
          <button class="button button--primary" type="button" @click="isCreateDialogOpen = true">
            <Plus :size="17" aria-hidden="true" />
            新建项目
          </button>
        </div>
      </div>
      <p v-if="importMessage" class="project-list__message">{{ importMessage }}</p>

      <div v-if="projectsQuery.isLoading.value" class="empty-state">正在加载项目</div>
      <div v-else-if="projects.length === 0" class="empty-state">暂无项目</div>
      <ul v-else class="project-items">
        <li v-for="project in projects" :key="project.id" class="project-row">
          <button class="project-row__open" type="button" @click="router.push({ name: 'workspace', params: { projectId: project.id } })">
            <BookOpen :size="18" aria-hidden="true" />
            <span>
              <strong>{{ project.name }}</strong>
              <small>{{ project.description || '无简介' }}</small>
            </span>
          </button>
          <button class="icon-button" type="button" title="删除项目" @click="deleteProject.mutate(project.id)">
            <Trash2 :size="17" aria-hidden="true" />
          </button>
        </li>
      </ul>
    </section>

    <div v-if="isCreateDialogOpen" class="dialog-backdrop" @click.self="isCreateDialogOpen = false">
      <section class="dialog" role="dialog" aria-modal="true" aria-labelledby="create-project-title">
        <header class="dialog__header">
          <h2 id="create-project-title">新建项目</h2>
          <button class="icon-button" type="button" title="关闭" @click="isCreateDialogOpen = false">
            <X :size="17" aria-hidden="true" />
          </button>
        </header>
        <form class="dialog-form" @submit.prevent="submitProject">
          <label>
            <span>名称</span>
            <input v-model="name" type="text" placeholder="项目名称" autofocus />
          </label>
          <label>
            <span>简介</span>
            <textarea v-model="description" rows="4" placeholder="项目简介" />
          </label>
          <footer class="dialog__footer">
            <button class="button" type="button" @click="isCreateDialogOpen = false">取消</button>
            <button class="button button--primary" type="submit" :disabled="createProject.isPending.value">
              <Plus :size="17" aria-hidden="true" />
              创建
            </button>
          </footer>
        </form>
      </section>
    </div>
  </main>
</template>
